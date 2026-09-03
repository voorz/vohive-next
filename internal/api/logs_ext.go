package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/voorz/vohive/pkg/logger"

	"github.com/gin-gonic/gin"
)

// logDatePattern 匹配日志文件名中的日期，如 app-2026-09-04.log
var logDatePattern = regexp.MustCompile(`app-(\d{4}-\d{2}-\d{2})\.log$`)

// handleLogDates 获取可用日志日期列表
//
// @Summary      获取可用日志日期列表
// @Tags         logs
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "日期列表"
// @Router       /logs/dates [get]
// @Security     BearerAuth
func (s *Server) handleLogDates(c *gin.Context) {
	logDir := "logs"

	entries, err := os.ReadDir(logDir)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"dates": []string{}})
		return
	}

	dates := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		m := logDatePattern.FindStringSubmatch(entry.Name())
		if len(m) == 2 {
			dates = append(dates, m[1])
		}
	}

	// 按日期降序排列（最新在前）
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	// 今天自动加入（即使当天日志文件还没有轮转）
	today := time.Now().Format("2006-01-02")
	hasToday := false
	for _, d := range dates {
		if d == today {
			hasToday = true
			break
		}
	}
	if !hasToday {
		dates = append([]string{today}, dates...)
	}

	c.JSON(http.StatusOK, gin.H{"dates": dates})
}

// handleLogHistoryByDate 按日期获取历史日志
//
// @Summary      按日期获取历史日志
// @Tags         logs
// @Produce      json
// @Param        date   query     string  false  "日期 (YYYY-MM-DD)，不传则读取当天"
// @Param        lines  query     int     false  "返回行数"  default(500)
// @Success      200    {object}  map[string]interface{}  "日志列表"
// @Router       /logs/history [get]
// @Security     BearerAuth
func (s *Server) handleLogHistoryByDate(c *gin.Context) {
	// 读取参数
	lines := 500
	if n := c.Query("lines"); n != "" {
		if v, err := strconv.Atoi(n); err == nil && v > 0 && v <= 2000 {
			lines = v
		}
	}

	date := strings.TrimSpace(c.Query("date"))

	// 确定日志文件路径
	var logFile string
	if date == "" {
		// 无日期参数：读取当天软链
		logFile = "logs/app.log"
	} else {
		// 有日期参数：读取指定日期的轮转文件
		logFile = fmt.Sprintf("logs/app-%s.log", date)
	}

	recentLines, err := readLastLines(logFile, lines, 1<<20)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"logs": []logger.LogEntry{}, "error": "无法读取日志文件"})
		return
	}

	// 解析日志行
	entries := make([]logger.LogEntry, 0, len(recentLines))
	for _, line := range recentLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entry := parseLogLine(line)
		if entry.Message != "" {
			entries = append(entries, entry)
		}
	}

	c.JSON(http.StatusOK, gin.H{"logs": entries})
}

// handleClearHistory 清理历史日志文件
//
// @Summary      清理历史日志文件
// @Tags         logs
// @Produce      json
// @Param        date    query     string  false  "指定日期 (YYYY-MM-DD)，删除该日期的日志文件"
// @Param        days    query     int     false  "删除最近 N 天之前的日志（保留最近 N 天），传 0 或不传则删除全部历史"
// @Param        keep    query     bool    false  "是否保留当天日志，默认 true"
// @Success      200    {object}  map[string]interface{}  "清理结果"
// @Router       /logs/history [delete]
// @Security     BearerAuth
func (s *Server) handleClearHistory(c *gin.Context) {
	logDir := "logs"
	date := strings.TrimSpace(c.Query("date"))
	keepToday := true
	if v := c.Query("keep"); v == "false" || v == "0" {
		keepToday = false
	}

	// 模式 1: 指定日期 — 删除单个文件
	if date != "" {
		target := filepath.Join(logDir, fmt.Sprintf("app-%s.log", date))
		if err := os.Remove(target); err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusOK, gin.H{"deleted": 0, "message": "文件不存在"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"deleted": 1, "message": "已删除 " + date + " 的日志"})
		return
	}

	// 模式 2: 按天数 — 删除 N 天之前的日志（保留最近 N 天）
	days := -1
	if v := c.Query("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			days = n
		}
	}

	// 模式 3: 无参数 — 删除全部历史日志（keepToday 控制是否保留当天）
	today := time.Now().Format("2006-01-02")
	cutoffDate := ""
	if days >= 0 {
		// 计算保留截止日期
		cutoff := time.Now().AddDate(0, 0, -days)
		cutoffDate = cutoff.Format("2006-01-02")
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取日志目录"})
		return
	}

	deleted := 0
	skipped := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		m := logDatePattern.FindStringSubmatch(entry.Name())
		if len(m) != 2 {
			continue
		}
		fileDate := m[1]

		// 保留当天
		if keepToday && fileDate == today {
			skipped++
			continue
		}

		// 按天数模式：保留 cutoffDate 之后的文件
		if days >= 0 && fileDate >= cutoffDate {
			skipped++
			continue
		}

		target := filepath.Join(logDir, entry.Name())
		if err := os.Remove(target); err == nil {
			deleted++
		}
	}

	msg := fmt.Sprintf("已清理 %d 个历史日志文件", deleted)
	if skipped > 0 {
		msg += fmt.Sprintf("，保留 %d 个", skipped)
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "skipped": skipped, "message": msg})
}
