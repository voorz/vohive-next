/**
 * 电话号码国家码 → ISO 映射
 * 从 eSIM Profile 名称（如 "+49 17430 38055"）解析出国旗 ISO 码
 */

// 常见国家电话码 → ISO 映射（覆盖主要地区）
const CODE_TO_ISO: Record<string, string> = {
  '1': 'us',
  '7': 'ru',
  '20': 'eg',
  '27': 'za',
  '30': 'gr',
  '31': 'nl',
  '32': 'be',
  '33': 'fr',
  '34': 'es',
  '36': 'hu',
  '39': 'it',
  '40': 'ro',
  '41': 'ch',
  '43': 'at',
  '44': 'gb',
  '45': 'dk',
  '46': 'se',
  '47': 'no',
  '48': 'pl',
  '49': 'de',
  '51': 'pe',
  '52': 'mx',
  '53': 'cu',
  '54': 'ar',
  '55': 'br',
  '56': 'cl',
  '57': 'co',
  '58': 've',
  '60': 'my',
  '61': 'au',
  '62': 'id',
  '63': 'ph',
  '64': 'nz',
  '65': 'sg',
  '66': 'th',
  '81': 'jp',
  '82': 'kr',
  '84': 'vn',
  '86': 'cn',
  '90': 'tr',
  '91': 'in',
  '92': 'pk',
  '93': 'af',
  '94': 'lk',
  '95': 'mm',
  '98': 'ir',
  '212': 'ma',
  '213': 'dz',
  '216': 'tn',
  '234': 'ng',
  '254': 'ke',
  '255': 'tz',
  '256': 'ug',
  '263': 'zw',
  '274': 'pt',
  '351': 'pt',
  '352': 'lu',
  '353': 'ie',
  '354': 'is',
  '358': 'fi',
  '359': 'bg',
  '370': 'lt',
  '371': 'lv',
  '372': 'ee',
  '374': 'am',
  '375': 'by',
  '376': 'ad',
  '377': 'mc',
  '378': 'sm',
  '380': 'ua',
  '381': 'rs',
  '385': 'hr',
  '386': 'si',
  '420': 'cz',
  '421': 'sk',
  '852': 'hk',
  '853': 'mo',
  '855': 'kh',
  '856': 'la',
  '880': 'bd',
  '886': 'tw',
  '960': 'mv',
  '961': 'lb',
  '962': 'jo',
  '963': 'sy',
  '964': 'iq',
  '965': 'kw',
  '966': 'sa',
  '971': 'ae',
  '972': 'il',
  '974': 'qa',
  '977': 'np',
  '994': 'az',
  '995': 'ge',
  '996': 'kg',
  '998': 'uz',
}

/**
 * 从电话号码解析国家 ISO 码
 * @param phone 如 "+49 17430 38055" 或 "+852 6686 6599"
 * @returns ISO 码如 "de"、"hk"，无法解析时返回空字符串
 */
export function phoneToIso(phone: string): string {
  if (!phone) return ''
  // 去掉 + 和空格
  const cleaned = phone.replace(/[\s\-()]/g, '')
  if (!cleaned.startsWith('+')) return ''

  const digits = cleaned.slice(1) // 去掉 +

  // 尝试 3位、2位、1位国家码
  for (const len of [3, 2, 1]) {
    if (digits.length <= len) continue
    const code = digits.slice(0, len)
    if (CODE_TO_ISO[code]) {
      return CODE_TO_ISO[code]
    }
  }

  return ''
}
