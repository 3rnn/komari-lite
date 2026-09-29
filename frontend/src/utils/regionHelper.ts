// Map region emojis to names.
export const emojiToRegionMap: Record<string, { en: string; zh: string; aliases: string[] }> = {
  '🇭🇰': {
    en: 'Hong Kong',
    zh: 'Hong Kong',
    aliases: ['hk', 'hongkong', 'hong kong', '\u9999\u6e2f', 'HK']
  },
  '🇨🇳': {
    en: 'China',
    zh: 'China',
    aliases: ['cn', 'china', '\u4e2d\u56fd', '\u4e2d\u534e\u4eba\u6c11\u5171\u548c\u56fd', 'prc', 'CN']
  },
  '🇺🇸': {
    en: 'United States',
    zh: 'United States',
    aliases: ['us', 'usa', 'united states', 'america', '\u7f8e\u56fd', '\u7f8e\u5229\u575a', 'US', 'USA']
  },
  '🇯🇵': {
    en: 'Japan',
    zh: 'Japan',
    aliases: ['jp', 'japan', '\u65e5\u672c', 'JP']
  },
  '🇰🇷': {
    en: 'South Korea',
    zh: 'South Korea',
    aliases: ['kr', 'korea', 'south korea', '\u97e9\u56fd', '\u5357\u97e9', 'KR']
  },
  '🇸🇬': {
    en: 'Singapore',
    zh: 'Singapore',
    aliases: ['sg', 'singapore', '\u65b0\u52a0\u5761', 'SG']
  },
  '🇹🇼': {
    en: 'Taiwan',
    zh: 'Taiwan',
    aliases: ['tw', 'taiwan', '\u53f0\u6e7e', '\u53f0\u7063', 'TW']
  },
  '🇬🇧': {
    en: 'United Kingdom',
    zh: 'United Kingdom',
    aliases: ['gb', 'uk', 'united kingdom', 'britain', '\u82f1\u56fd', '\u82f1\u570b', 'GB', 'UK']
  },
  '🇩🇪': {
    en: 'Germany',
    zh: 'Germany',
    aliases: ['de', 'germany', 'deutschland', '\u5fb7\u56fd', '\u5fb7\u570b', 'DE']
  },
  '🇫🇷': {
    en: 'France',
    zh: 'France',
    aliases: ['fr', 'france', '\u6cd5\u56fd', '\u6cd5\u570b', 'FR']
  },
  '🇨🇦': {
    en: 'Canada',
    zh: 'Canada',
    aliases: ['ca', 'canada', '\u52a0\u62ff\u5927', 'CA']
  },
  '🇦🇺': {
    en: 'Australia',
    zh: 'Australia',
    aliases: ['au', 'australia', '\u6fb3\u5927\u5229\u4e9a', '\u6fb3\u6d32', 'AU']
  },
  '🇷🇺': {
    en: 'Russia',
    zh: 'Russia',
    aliases: ['ru', 'russia', '\u4fc4\u7f57\u65af', '\u4fc4\u570b', 'RU']
  },
  '🇮🇳': {
    en: 'India',
    zh: 'India',
    aliases: ['in', 'india', '\u5370\u5ea6', 'IN']
  },
  '🇧🇷': {
    en: 'Brazil',
    zh: 'Brazil',
    aliases: ['br', 'brazil', '\u5df4\u897f', 'BR']
  },
  '🇳🇱': {
    en: 'Netherlands',
    zh: 'Netherlands',
    aliases: ['nl', 'netherlands', 'holland', '\u8377\u5170', '\u8377\u862d', 'NL']
  },
  '🇮🇹': {
    en: 'Italy',
    zh: 'Italy',
    aliases: ['it', 'italy', '\u610f\u5927\u5229', 'IT']
  },
  '🇪🇸': {
    en: 'Spain',
    zh: 'Spain',
    aliases: ['es', 'spain', '\u897f\u73ed\u7259', 'ES']
  },
  '🇸🇪': {
    en: 'Sweden',
    zh: 'Sweden',
    aliases: ['se', 'sweden', '\u745e\u5178', 'SE']
  },
  '🇳🇴': {
    en: 'Norway',
    zh: 'Norway',
    aliases: ['no', 'norway', '\u632a\u5a01', 'NO']
  },
  '🇫🇮': {
    en: 'Finland',
    zh: 'Finland',
    aliases: ['fi', 'finland', '\u82ac\u5170', '\u82ac\u862d', 'FI']
  },
  '🇨🇭': {
    en: 'Switzerland',
    zh: 'Switzerland',
    aliases: ['ch', 'switzerland', '\u745e\u58eb', 'CH']
  },
  '🇦🇹': {
    en: 'Austria',
    zh: 'Austria',
    aliases: ['at', 'austria', '\u5965\u5730\u5229', '\u5967\u5730\u5229', 'AT']
  },
  '🇧🇪': {
    en: 'Belgium',
    zh: 'Belgium',
    aliases: ['be', 'belgium', '\u6bd4\u5229\u65f6', '\u6bd4\u5229\u6642', 'BE']
  },
  '🇵🇹': {
    en: 'Portugal',
    zh: 'Portugal',
    aliases: ['pt', 'portugal', '\u8461\u8404\u7259', 'PT']
  },
  '🇬🇷': {
    en: 'Greece',
    zh: 'Greece',
    aliases: ['gr', 'greece', '\u5e0c\u814a', '\u5e0c\u81d8', 'GR']
  },
  '🇹🇷': {
    en: 'Turkey',
    zh: 'Turkey',
    aliases: ['tr', 'turkey', '\u571f\u8033\u5176', 'TR']
  },
  '🇵🇱': {
    en: 'Poland',
    zh: 'Poland',
    aliases: ['pl', 'poland', '\u6ce2\u5170', '\u6ce2\u862d', 'PL']
  },
  '🇨🇿': {
    en: 'Czech Republic',
    zh: 'Czech Republic',
    aliases: ['cz', 'czech', 'czech republic', '\u6377\u514b', 'CZ']
  },
  '🇭🇺': {
    en: 'Hungary',
    zh: 'Hungary',
    aliases: ['hu', 'hungary', '\u5308\u7259\u5229', 'HU']
  },
  '🇷🇴': {
    en: 'Romania',
    zh: 'Romania',
    aliases: ['ro', 'romania', '\u7f57\u9a6c\u5c3c\u4e9a', '\u7f85\u99ac\u5c3c\u4e9e', 'RO']
  },
  '🇧🇬': {
    en: 'Bulgaria',
    zh: 'Bulgaria',
    aliases: ['bg', 'bulgaria', '\u4fdd\u52a0\u5229\u4e9a', '\u4fdd\u52a0\u5229\u4e9e', 'BG']
  },
  '🇭🇷': {
    en: 'Croatia',
    zh: 'Croatia',
    aliases: ['hr', 'croatia', '\u514b\u7f57\u5730\u4e9a', '\u514b\u7f85\u5730\u4e9e', 'HR']
  },
  '🇸🇮': {
    en: 'Slovenia',
    zh: 'Slovenia',
    aliases: ['si', 'slovenia', '\u65af\u6d1b\u6587\u5c3c\u4e9a', '\u65af\u6d1b\u6587\u5c3c\u4e9e', 'SI']
  },
  '🇸🇰': {
    en: 'Slovakia',
    zh: 'Slovakia',
    aliases: ['sk', 'slovakia', '\u65af\u6d1b\u4f10\u514b', 'SK']
  },
  '🇱🇻': {
    en: 'Latvia',
    zh: 'Latvia',
    aliases: ['lv', 'latvia', '\u62c9\u8131\u7ef4\u4e9a', '\u62c9\u812b\u7dad\u4e9e', 'LV']
  },
  '🇱🇹': {
    en: 'Lithuania',
    zh: 'Lithuania',
    aliases: ['lt', 'lithuania', '\u7acb\u9676\u5b9b', 'LT']
  },
  '🇪🇪': {
    en: 'Estonia',
    zh: 'Estonia',
    aliases: ['ee', 'estonia', '\u7231\u6c99\u5c3c\u4e9a', '\u611b\u6c99\u5c3c\u4e9e', 'EE']
  },
  '🇲🇽': {
    en: 'Mexico',
    zh: 'Mexico',
    aliases: ['mx', 'mexico', '\u58a8\u897f\u54e5', 'MX']
  },
  '🇦🇷': {
    en: 'Argentina',
    zh: 'Argentina',
    aliases: ['ar', 'argentina', '\u963f\u6839\u5ef7', 'AR']
  },
  '🇨🇱': {
    en: 'Chile',
    zh: 'Chile',
    aliases: ['cl', 'chile', '\u667a\u5229', 'CL']
  },
  '🇨🇴': {
    en: 'Colombia',
    zh: 'Colombia',
    aliases: ['co', 'colombia', '\u54e5\u4f26\u6bd4\u4e9a', '\u54e5\u502b\u6bd4\u4e9e', 'CO']
  },
  '🇵🇪': {
    en: 'Peru',
    zh: 'Peru',
    aliases: ['pe', 'peru', '\u79d8\u9c81', '\u79d8\u9b6f', 'PE']
  },
  '🇻🇪': {
    en: 'Venezuela',
    zh: 'Venezuela',
    aliases: ['ve', 'venezuela', '\u59d4\u5185\u745e\u62c9', '\u59d4\u5167\u745e\u62c9', 'VE']
  },
  '🇺🇾': {
    en: 'Uruguay',
    zh: 'Uruguay',
    aliases: ['uy', 'uruguay', '\u4e4c\u62c9\u572d', '\u70cf\u62c9\u572d', 'UY']
  },
  '🇪🇨': {
    en: 'Ecuador',
    zh: 'Ecuador',
    aliases: ['ec', 'ecuador', '\u5384\u74dc\u591a\u5c14', '\u5384\u74dc\u591a\u723e', 'EC']
  },
  '🇧🇴': {
    en: 'Bolivia',
    zh: 'Bolivia',
    aliases: ['bo', 'bolivia', '\u73bb\u5229\u7ef4\u4e9a', '\u73bb\u5229\u7dad\u4e9e', 'BO']
  },
  '🇵🇾': {
    en: 'Paraguay',
    zh: 'Paraguay',
    aliases: ['py', 'paraguay', '\u5df4\u62c9\u572d', 'PY']
  },
  '🇬🇾': {
    en: 'Guyana',
    zh: 'Guyana',
    aliases: ['gy', 'guyana', '\u572d\u4e9a\u90a3', '\u572d\u4e9e\u90a3', 'GY']
  },
  '🇸🇷': {
    en: 'Suriname',
    zh: 'Suriname',
    aliases: ['sr', 'suriname', '\u82cf\u91cc\u5357', '\u8607\u91cc\u5357', 'SR']
  },
  '🇫🇰': {
    en: 'Falkland Islands',
    zh: 'Falkland Islands',
    aliases: ['fk', 'falkland', '\u798f\u514b\u5170', '\u798f\u514b\u862d', 'FK']
  },
  '🇬🇫': {
    en: 'French Guiana',
    zh: 'French Guiana',
    aliases: ['gf', 'french guiana', '\u6cd5\u5c5e\u572d\u4e9a\u90a3', '\u6cd5\u5c6c\u572d\u4e9e\u90a3', 'GF']
  },
  '🇵🇦': {
    en: 'Panama',
    zh: 'Panama',
    aliases: ['pa', 'panama', '\u5df4\u62ff\u9a6c', '\u5df4\u62ff\u99ac', 'PA']
  },
  '🇨🇷': {
    en: 'Costa Rica',
    zh: 'Costa Rica',
    aliases: ['cr', 'costa rica', '\u54e5\u65af\u8fbe\u9ece\u52a0', '\u54e5\u65af\u9054\u9ece\u52a0', 'CR']
  },
  '🇳🇮': {
    en: 'Nicaragua',
    zh: 'Nicaragua',
    aliases: ['ni', 'nicaragua', '\u5c3c\u52a0\u62c9\u74dc', 'NI']
  },
  '🇭🇳': {
    en: 'Honduras',
    zh: 'Honduras',
    aliases: ['hn', 'honduras', '\u6d2a\u90fd\u62c9\u65af', 'HN']
  },
  '🇬🇹': {
    en: 'Guatemala',
    zh: 'Guatemala',
    aliases: ['gt', 'guatemala', '\u5371\u5730\u9a6c\u62c9', '\u5371\u5730\u99ac\u62c9', 'GT']
  },
  '🇧🇿': {
    en: 'Belize',
    zh: 'Belize',
    aliases: ['bz', 'belize', '\u4f2f\u5229\u5179', '\u4f2f\u5229\u8332', 'BZ']
  },
  '🇸🇻': {
    en: 'El Salvador',
    zh: 'El Salvador',
    aliases: ['sv', 'el salvador', '\u8428\u5c14\u74e6\u591a', '\u85a9\u723e\u74e6\u591a', 'SV']
  },
  '🇯🇲': {
    en: 'Jamaica',
    zh: 'Jamaica',
    aliases: ['jm', 'jamaica', '\u7259\u4e70\u52a0', '\u7259\u8cb7\u52a0', 'JM']
  },
  '🇨🇺': {
    en: 'Cuba',
    zh: 'Cuba',
    aliases: ['cu', 'cuba', '\u53e4\u5df4', 'CU']
  },
  '🇩🇴': {
    en: 'Dominican Republic',
    zh: 'Dominican Republic',
    aliases: ['do', 'dominican', '\u591a\u660e\u5c3c\u52a0', 'DO']
  },
  '🇭🇹': {
    en: 'Haiti',
    zh: 'Haiti',
    aliases: ['ht', 'haiti', '\u6d77\u5730', 'HT']
  },
  '🇧🇸': {
    en: 'Bahamas',
    zh: 'Bahamas',
    aliases: ['bs', 'bahamas', '\u5df4\u54c8\u9a6c', '\u5df4\u54c8\u99ac', 'BS']
  },
  '🇧🇧': {
    en: 'Barbados',
    zh: 'Barbados',
    aliases: ['bb', 'barbados', '\u5df4\u5df4\u591a\u65af', 'BB']
  },
  '🇹🇹': {
    en: 'Trinidad and Tobago',
    zh: 'Trinidad and Tobago',
    aliases: ['tt', 'trinidad', '\u7279\u7acb\u5c3c\u8fbe', '\u7279\u7acb\u5c3c\u9054', 'TT']
  },
  '🇵🇭': {
    en: 'Philippines',
    zh: 'Philippines',
    aliases: ['ph', 'philippines', '\u83f2\u5f8b\u5bbe', '\u83f2\u5f8b\u8cd3', 'PH']
  },
  '🇹🇭': {
    en: 'Thailand',
    zh: 'Thailand',
    aliases: ['th', 'thailand', '\u6cf0\u56fd', '\u6cf0\u570b', 'TH']
  },
  '🇻🇳': {
    en: 'Vietnam',
    zh: 'Vietnam',
    aliases: ['vn', 'vietnam', '\u8d8a\u5357', 'VN']
  },
  '🇲🇾': {
    en: 'Malaysia',
    zh: 'Malaysia',
    aliases: ['my', 'malaysia', '\u9a6c\u6765\u897f\u4e9a', '\u99ac\u4f86\u897f\u4e9e', 'MY']
  },
  '🇮🇩': {
    en: 'Indonesia',
    zh: 'Indonesia',
    aliases: ['id', 'indonesia', '\u5370\u5ea6\u5c3c\u897f\u4e9a', '\u5370\u5c3c', 'ID']
  },
  '🇱🇦': {
    en: 'Laos',
    zh: 'Laos',
    aliases: ['la', 'laos', '\u8001\u631d', '\u8001\u64be', 'LA']
  },
  '🇰🇭': {
    en: 'Cambodia',
    zh: 'Cambodia',
    aliases: ['kh', 'cambodia', '\u67ec\u57d4\u5be8', 'KH']
  },
  '🇲🇲': {
    en: 'Myanmar',
    zh: 'Myanmar',
    aliases: ['mm', 'myanmar', 'burma', '\u7f05\u7538', '\u7dec\u7538', 'MM']
  },
  '🇧🇳': {
    en: 'Brunei',
    zh: 'Brunei',
    aliases: ['bn', 'brunei', '\u6587\u83b1', '\u6c76\u840a', 'BN']
  },
  '🇪🇬': {
    en: 'Egypt',
    zh: 'Egypt',
    aliases: ['eg', 'egypt', '\u57c3\u53ca', 'EG']
  },
  '🇿🇦': {
    en: 'South Africa',
    zh: 'South Africa',
    aliases: ['za', 'south africa', '\u5357\u975e', 'ZA']
  },
  '🇳🇬': {
    en: 'Nigeria',
    zh: 'Nigeria',
    aliases: ['ng', 'nigeria', '\u5c3c\u65e5\u5229\u4e9a', '\u5c3c\u65e5\u5229\u4e9e', 'NG']
  },
  '🇰🇪': {
    en: 'Kenya',
    zh: 'Kenya',
    aliases: ['ke', 'kenya', '\u80af\u5c3c\u4e9a', '\u80af\u4e9e', 'KE']
  },
  '🇪🇹': {
    en: 'Ethiopia',
    zh: 'Ethiopia',
    aliases: ['et', 'ethiopia', '\u57c3\u585e\u4fc4\u6bd4\u4e9a', '\u57c3\u585e\u4fc4\u6bd4\u4e9e', 'ET']
  },
  '🇬🇭': {
    en: 'Ghana',
    zh: 'Ghana',
    aliases: ['gh', 'ghana', '\u52a0\u7eb3', '\u8fe6\u7d0d', 'GH']
  },
  '🇺🇬': {
    en: 'Uganda',
    zh: 'Uganda',
    aliases: ['ug', 'uganda', '\u4e4c\u5e72\u8fbe', '\u70cf\u5e72\u9054', 'UG']
  },
  '🇹🇿': {
    en: 'Tanzania',
    zh: 'Tanzania',
    aliases: ['tz', 'tanzania', '\u5766\u6851\u5c3c\u4e9a', '\u5766\u5c1a\u5c3c\u4e9e', 'TZ']
  },
  '🇷🇼': {
    en: 'Rwanda',
    zh: 'Rwanda',
    aliases: ['rw', 'rwanda', '\u5362\u65fa\u8fbe', '\u76e7\u65fa\u9054', 'RW']
  },
  '🇿🇼': {
    en: 'Zimbabwe',
    zh: 'Zimbabwe',
    aliases: ['zw', 'zimbabwe', '\u6d25\u5df4\u5e03\u97e6', '\u8f9b\u5df4\u5a01', 'ZW']
  },
  '🇿🇲': {
    en: 'Zambia',
    zh: 'Zambia',
    aliases: ['zm', 'zambia', '\u8d5e\u6bd4\u4e9a', '\u5c1a\u6bd4\u4e9e', 'ZM']
  },
  '🇧🇼': {
    en: 'Botswana',
    zh: 'Botswana',
    aliases: ['bw', 'botswana', '\u535a\u8328\u74e6\u7eb3', '\u6ce2\u672d\u90a3', 'BW']
  },
  '🇳🇦': {
    en: 'Namibia',
    zh: 'Namibia',
    aliases: ['na', 'namibia', '\u7eb3\u7c73\u6bd4\u4e9a', '\u7d0d\u7c73\u6bd4\u4e9e', 'NA']
  },
  '🇲🇦': {
    en: 'Morocco',
    zh: 'Morocco',
    aliases: ['ma', 'morocco', '\u6469\u6d1b\u54e5', 'MA']
  },
  '🇩🇿': {
    en: 'Algeria',
    zh: 'Algeria',
    aliases: ['dz', 'algeria', '\u963f\u5c14\u53ca\u5229\u4e9a', '\u963f\u723e\u53ca\u5229\u4e9e', 'DZ']
  },
  '🇹🇳': {
    en: 'Tunisia',
    zh: 'Tunisia',
    aliases: ['tn', 'tunisia', '\u7a81\u5c3c\u65af', 'TN']
  },
  '🇱🇾': {
    en: 'Libya',
    zh: 'Libya',
    aliases: ['ly', 'libya', '\u5229\u6bd4\u4e9a', '\u5229\u6bd4\u4e9e', 'LY']
  },
  '🇸🇩': {
    en: 'Sudan',
    zh: 'Sudan',
    aliases: ['sd', 'sudan', '\u82cf\u4e39', '\u8607\u4e39', 'SD']
  },
  '🇸🇸': {
    en: 'South Sudan',
    zh: 'South Sudan',
    aliases: ['ss', 'south sudan', '\u5357\u82cf\u4e39', '\u5357\u8607\u4e39', 'SS']
  },
  '🇨🇩': {
    en: 'Democratic Republic of Congo',
    zh: 'Democratic Republic of Congo',
    aliases: ['cd', 'congo', 'drc', '\u521a\u679c', '\u525b\u679c', 'CD']
  },
  '🇨🇬': {
    en: 'Republic of Congo',
    zh: 'Republic of Congo',
    aliases: ['cg', 'congo', '\u521a\u679c', '\u525b\u679c', 'CG']
  },
  '🇨🇫': {
    en: 'Central African Republic',
    zh: 'Central African Republic',
    aliases: ['cf', 'central african', '\u4e2d\u975e', 'CF']
  },
  '🇨🇲': {
    en: 'Cameroon',
    zh: 'Cameroon',
    aliases: ['cm', 'cameroon', '\u5580\u9ea6\u9686', '\u5580\u9ea5\u9686', 'CM']
  },
  '🇹🇩': {
    en: 'Chad',
    zh: 'Chad',
    aliases: ['td', 'chad', '\u4e4d\u5f97', 'TD']
  },
  '🇳🇪': {
    en: 'Niger',
    zh: 'Niger',
    aliases: ['ne', 'niger', '\u5c3c\u65e5\u5c14', '\u5c3c\u65e5\u723e', 'NE']
  },
  '🇲🇱': {
    en: 'Mali',
    zh: 'Mali',
    aliases: ['ml', 'mali', '\u9a6c\u91cc', '\u99ac\u5229', 'ML']
  },
  '🇧🇫': {
    en: 'Burkina Faso',
    zh: 'Burkina Faso',
    aliases: ['bf', 'burkina', '\u5e03\u57fa\u7eb3\u6cd5\u7d22', '\u5e03\u5409\u7d0d\u6cd5\u7d22', 'BF']
  },
  '🇸🇳': {
    en: 'Senegal',
    zh: 'Senegal',
    aliases: ['sn', 'senegal', '\u585e\u5185\u52a0\u5c14', '\u585e\u5167\u52a0\u723e', 'SN']
  },
  '🇬🇲': {
    en: 'Gambia',
    zh: 'Gambia',
    aliases: ['gm', 'gambia', '\u5188\u6bd4\u4e9a', '\u7518\u6bd4\u4e9e', 'GM']
  },
  '🇬🇼': {
    en: 'Guinea-Bissau',
    zh: 'Guinea-Bissau',
    aliases: ['gw', 'guinea-bissau', '\u51e0\u5185\u4e9a\u6bd4\u7ecd', '\u5e7e\u5167\u4e9e\u6bd4\u7d22', 'GW']
  },
  '🇬🇳': {
    en: 'Guinea',
    zh: 'Guinea',
    aliases: ['gn', 'guinea', '\u51e0\u5185\u4e9a', '\u5e7e\u5167\u4e9e', 'GN']
  },
  '🇸🇱': {
    en: 'Sierra Leone',
    zh: 'Sierra Leone',
    aliases: ['sl', 'sierra leone', '\u585e\u62c9\u5229\u6602', 'SL']
  },
  '🇱🇷': {
    en: 'Liberia',
    zh: 'Liberia',
    aliases: ['lr', 'liberia', '\u5229\u6bd4\u91cc\u4e9a', '\u8cf4\u6bd4\u745e\u4e9e', 'LR']
  },
  '🇨🇮': {
    en: 'Ivory Coast',
    zh: 'Ivory Coast',
    aliases: ['ci', 'ivory coast', '\u79d1\u7279\u8fea\u74e6', '\u8c61\u7259\u6d77\u5cb8', 'CI']
  },
  '🇹🇬': {
    en: 'Togo',
    zh: 'Togo',
    aliases: ['tg', 'togo', '\u591a\u54e5', 'TG']
  },
  '🇧🇯': {
    en: 'Benin',
    zh: 'Benin',
    aliases: ['bj', 'benin', '\u8d1d\u5b81', '\u8c9d\u5be7', 'BJ']
  }
};

/**
 * Check whether a region emoji matches a search term.
 * @param regionEmoji Region emoji (e.g. 🇭🇰).
 * @param searchTerm Search term.
 * @returns Whether the region matches.
 */
export const isRegionMatch = (regionEmoji: string, searchTerm: string): boolean => {
  const lowerSearchTerm = searchTerm.toLowerCase().trim();
  
  // Match the emoji directly.
  if (regionEmoji === searchTerm) {
    return true;
  }
  
  // Look up the emoji in the region map.
  const regionInfo = emojiToRegionMap[regionEmoji];
  if (!regionInfo) {
    // Without a map entry, only try a simple substring match.
    return regionEmoji.toLowerCase().includes(lowerSearchTerm);
  }
  
  // Check the English name.
  if (regionInfo.en.toLowerCase().includes(lowerSearchTerm)) {
    return true;
  }
  
  // Check the legacy-language name.
  if (regionInfo.zh.includes(lowerSearchTerm)) {
    return true;
  }
  
  // Check aliases, including encoded legacy names.
  return regionInfo.aliases.some(alias => 
    alias.toLowerCase().includes(lowerSearchTerm)
  );
};

/**
 * Get the region's display name.
 * @param regionEmoji Region emoji.
 * @param language Language code ('en' | 'zh').
 * @returns Region name.
 */
export const getRegionDisplayName = (regionEmoji: string, language: 'en' | 'zh' = 'zh'): string => {
  const regionInfo = emojiToRegionMap[regionEmoji];
  if (!regionInfo) {
    return regionEmoji;
  }
  
  return language === 'zh' ? regionInfo.zh : regionInfo.en;
};

/** Convert a flag emoji or an existing ISO value to a two-letter region code. */
export const getRegionCode = (region?: string | null): string => {
  const normalized = typeof region === "string" ? region.trim() : "";
  if (/^[a-z]{2}$/i.test(normalized)) {
    return normalized.toUpperCase();
  }

  const indicators = Array.from(normalized);
  if (indicators.length !== 2) {
    return "UN";
  }

  const start = 0x1f1e6;
  const codePoints = indicators.map((indicator) => indicator.codePointAt(0) ?? 0);
  if (codePoints.some((codePoint) => codePoint < start || codePoint > 0x1f1ff)) {
    return "UN";
  }

  return codePoints
    .map((codePoint) => String.fromCharCode(0x41 + codePoint - start))
    .join("");
};

/**
 * Get all supported region emojis.
 * @returns Array of region emojis.
 */
export const getSupportedRegions = (): string[] => {
  return Object.keys(emojiToRegionMap);
};
