package clients

import "strings"

// iso3166Alpha2 is the complete ISO 3166-1 alpha-2 assignment set used by the
// administrative country override API. It includes officially assigned
// territories, matches the frontend selector, and intentionally stores GB
// rather than the colloquial UK alias.
const iso3166Alpha2 = "AD AE AF AG AI AL AM AO AQ AR AS AT AU AW AX AZ BA BB BD BE BF BG BH BI BJ BL BM BN BO BQ BR BS BT BV BW BY BZ CA CC CD CF CG CH CI CK CL CM CN CO CR CU CV CW CX CY CZ DE DJ DK DM DO DZ EC EE EG EH ER ES ET FI FJ FK FM FO FR GA GB GD GE GF GG GH GI GL GM GN GP GQ GR GS GT GU GW GY HK HM HN HR HT HU ID IE IL IM IN IO IQ IR IS IT JE JM JO JP KE KG KH KI KM KN KP KR KW KY KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR MS MT MU MV MW MX MY MZ NA NC NE NF NG NI NL NO NP NR NU NZ OM PA PE PF PG PH PK PL PM PN PR PS PT PW PY QA RE RO RS RU RW SA SB SC SD SE SG SH SI SJ SK SL SM SN SO SR SS ST SV SX SY SZ TC TD TF TG TH TJ TK TL TM TN TO TR TT TV TW TZ UA UG UM US UY UZ VA VC VE VG VI VN VU WF WS YE YT ZA ZM ZW"

func canonicalISO3166Alpha2(value string) (string, bool) {
	code := strings.ToUpper(strings.TrimSpace(value))
	if code == "UK" {
		code = "GB"
	}
	if len(code) != 2 || code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
		return "", false
	}
	return code, strings.Contains(" "+iso3166Alpha2+" ", " "+code+" ")
}

func iso3166Flag(code string) string {
	return string(rune(0x1F1E6+int(code[0]-'A'))) + string(rune(0x1F1E6+int(code[1]-'A')))
}

func iso3166CodeFromFlag(value string) (string, bool) {
	runes := []rune(value)
	if len(runes) != 2 || runes[0] < 0x1F1E6 || runes[0] > 0x1F1FF || runes[1] < 0x1F1E6 || runes[1] > 0x1F1FF {
		return "", false
	}
	return canonicalISO3166Alpha2(string([]byte{byte('A' + runes[0] - 0x1F1E6), byte('A' + runes[1] - 0x1F1E6)}))
}
