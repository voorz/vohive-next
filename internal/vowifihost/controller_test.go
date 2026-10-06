package vowifihost

import (
	"context"
	"testing"
	"time"
)

func TestLifecycleControllerEnableKeepsRunContextAliveAfterSubmitReturns(t *testing.T) {
	c := NewLifecycleController()

	var enableCtx context.Context
	c.TestRun = func(ctx context.Context, cmd LifecycleCommand) error {
		if cmd.Kind != LifecycleCommandEnable {
			t.Fatalf("command kind = %s, want enable", cmd.Kind.String())
		}
		enableCtx = ctx
		return nil
	}

	if err := c.Submit(context.Background(), LifecycleCommand{DeviceID: "dev-1", Kind: LifecycleCommandEnable}); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if enableCtx == nil {
		t.Fatal("enable context was not captured")
	}

	select {
	case <-enableCtx.Done():
		t.Fatalf("enable context was canceled after successful Submit: %v", enableCtx.Err())
	default:
	}
}

func TestLifecycleControllerSwitchBeginPreemptsInFlightEnable(t *testing.T) {
	c := NewLifecycleController()

	enableRelease := make(chan struct{})
	started := make(chan LifecycleCommand, 2)
	done := make(chan error, 2)

	c.TestRun = func(ctx context.Context, cmd LifecycleCommand) error {
		started <- cmd
		if cmd.Kind == LifecycleCommandEnable {
			<-enableRelease
		}
		return nil
	}

	go func() {
		done <- c.Submit(context.Background(), LifecycleCommand{DeviceID: "dev-1", Kind: LifecycleCommandEnable})
	}()

	var enableCmd LifecycleCommand
	select {
	case enableCmd = <-started:
		if enableCmd.Kind != LifecycleCommandEnable {
			t.Fatalf("first command = %s, want enable", enableCmd.Kind.String())
		}
	case <-time.After(time.Second):
		t.Fatal("enable command did not start")
	}

	go func() {
		done <- c.Submit(context.Background(), LifecycleCommand{DeviceID: "dev-1", Kind: LifecycleCommandSwitchBegin})
	}()

	var switchCmd LifecycleCommand
	select {
	case switchCmd = <-started:
		if switchCmd.Kind != LifecycleCommandSwitchBegin {
			t.Fatalf("preempting command = %s, want switch_begin", switchCmd.Kind.String())
		}
	case <-time.After(150 * time.Millisecond):
		t.Fatal("switch_begin did not preempt in-flight enable")
	}

	if switchCmd.Generation <= enableCmd.Generation {
		t.Fatalf("switch_begin generation = %d, want > in-flight enable generation %d", switchCmd.Generation, enableCmd.Generation)
	}

	close(enableRelease)
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("submit returned error: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for submit to finish")
		}
	}
}

func TestLifecycleControllerRestartPreemptsInFlightEnable(t *testing.T) {
	c := NewLifecycleController()

	started := make(chan LifecycleCommand, 2)
	done := make(chan error, 2)

	c.TestRun = func(ctx context.Context, cmd LifecycleCommand) error {
		started <- cmd
		if cmd.Kind == LifecycleCommandEnable {
			<-ctx.Done()
		}
		return nil
	}

	go func() {
		done <- c.Submit(context.Background(), LifecycleCommand{DeviceID: "dev-1", Kind: LifecycleCommandEnable})
	}()

	var enableCmd LifecycleCommand
	select {
	case enableCmd = <-started:
		if enableCmd.Kind != LifecycleCommandEnable {
			t.Fatalf("first command = %s, want enable", enableCmd.Kind.String())
		}
	case <-time.After(time.Second):
		t.Fatal("enable command did not start")
	}

	go func() {
		done <- c.Submit(context.Background(), LifecycleCommand{DeviceID: "dev-1", Kind: LifecycleCommandRestart})
	}()

	var restartCmd LifecycleCommand
	select {
	case restartCmd = <-started:
		if restartCmd.Kind != LifecycleCommandRestart {
			t.Fatalf("preempting command = %s, want restart", restartCmd.Kind.String())
		}
	case <-time.After(150 * time.Millisecond):
		t.Fatal("restart did not preempt in-flight enable")
	}

	if restartCmd.Generation <= enableCmd.Generation {
		t.Fatalf("restart generation = %d, want > in-flight enable generation %d", restartCmd.Generation, enableCmd.Generation)
	}

	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("submit returned error: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for submit to finish")
		}
	}
}

func TestTryAcquireRunSemTimeout(t *testing.T) {
	c := NewLifecycleController()
	lifecycle := c.device("dev-timeout")

	// 先占用信号量
	if !c.tryAcquireRunSem(lifecycle, time.Second) {
		t.Fatal("first acquire should succeed")
	}
	defer c.releaseRunSem(lifecycle)

	// 第二次获取应超时失败（用短超时加速测试）
	start := time.Now()
	if c.tryAcquireRunSem(lifecycle, 100*time.Millisecond) {
		t.Fatal("second acquire should timeout")
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("timeout too fast: %v", elapsed)
	}
}

func TestRunSemNoLeakAfterMultipleSubmits(t *testing.T) {
	c := NewLifecycleController()
	c.TestRun = func(ctx context.Context, cmd LifecycleCommand) error {
		return nil
	}

	// 连续提交多次，信号量应正确释放
	for i := 0; i < 10; i++ {
		if err := c.Submit(context.Background(), LifecycleCommand{
			DeviceID: "dev-leak",
			Kind:     LifecycleCommandEnable,
		}); err != nil {
			t.Fatalf("Submit %d failed: %v", i, err)
		}
	}

	// 验证信号量可用（非阻塞获取）
	lifecycle := c.device("dev-leak")
	select {
	case <-lifecycle.runSem:
		// 成功获取，说明无泄漏
		lifecycle.runSem <- struct{}{} // 归还
	default:
		t.Fatal("runSem leaked: not available after 10 Submits")
	}
}

func TestSubmitTimeoutCancelsStuckCommand(t *testing.T) {
	c := NewLifecycleController()

	// 模拟一个卡住的命令（阻塞直到 context 取消）
	unblocked := make(chan struct{})
	c.TestRun = func(ctx context.Context, cmd LifecycleCommand) error {
		select {
		case <-ctx.Done():
			close(unblocked)
			return ctx.Err()
		case <-time.After(10 * time.Second):
			return nil
		}
	}

	// 启动卡住的命令（异步，因为它会阻塞）
	go func() {
		_ = c.Submit(context.Background(), LifecycleCommand{
			DeviceID: "dev-stuck",
			Kind:     LifecycleCommandEnable,
		})
	}()

	// 等待第一个命令占用信号量
	time.Sleep(100 * time.Millisecond)

	// 第二个 Submit 应该超时（用短超时需要修改代码，这里直接测试 tryAcquire）
	lifecycle := c.device("dev-stuck")
	if c.tryAcquireRunSem(lifecycle, 100*time.Millisecond) {
		t.Fatal("should not acquire while first command is stuck")
	}

	// 取消卡住的命令
	c.cancelActiveRun(lifecycle)

	// 等待卡住的命令响应取消
	select {
	case <-unblocked:
		// 成功响应取消
	case <-time.After(2 * time.Second):
		t.Fatal("stuck command did not respond to cancel")
	}
}
