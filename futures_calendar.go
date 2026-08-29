// futures_calendar 提供 MAC 期货 K 线交易日到自然时间的 SQLite 转换。
package gotdx

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bensema/gotdx/proto"
	_ "modernc.org/sqlite"
)

const futuresCalendarLocation = "Asia/Shanghai"

// resolveMACFuturesDateTimes 用期货交易日历将 MAC 期货 K 线转换为自然时间。
func resolveMACFuturesDateTimes(items []proto.MACSymbolBar, code, calendarPath string) error {
	exchange, ok := futuresExchange(code)
	if !ok || calendarPath == "" {
		return nil
	}

	db, err := sql.Open("sqlite", calendarPath)
	if err != nil {
		return fmt.Errorf("open futures trade calendar: %w", err)
	}
	defer db.Close()

	preTradeDates := make(map[uint32]time.Time)
	for index := range items {
		seconds := items[index].RawSeconds
		if !isMACNightSession(seconds) {
			continue
		}
		preTradeDate, found := preTradeDates[items[index].RawYMD]
		if !found {
			preTradeDate, err = futuresPreTradeDate(db, exchange, items[index].RawYMD)
			if err != nil {
				return err
			}
			preTradeDates[items[index].RawYMD] = preTradeDate
		}

		if seconds >= 21*60*60 {
			items[index].DateTime = combineFuturesDateTime(preTradeDate, seconds)
			continue
		}
		items[index].DateTime = combineFuturesDateTime(preTradeDate.AddDate(0, 0, 1), seconds)
	}
	return nil
}

// futuresExchange 根据期货合约代码前缀映射 Tushare 交易所代码。
func futuresExchange(code string) (string, bool) {
	prefix := strings.TrimRight(strings.ToUpper(strings.TrimSpace(code)), "0123456789")
	switch prefix {
	case "AL", "AO", "AG", "AU", "BR", "BU", "CU", "FU", "HC", "NI", "PB", "RB", "RU", "SN", "SP", "SS", "WR", "ZN":
		return "SHFE", true
	case "BC", "EC", "LU", "NR", "SC":
		return "INE", true
	case "A", "B", "C", "CS", "EB", "EG", "FB", "I", "J", "JD", "JM", "L", "LH", "M", "P", "PG", "PP", "RR", "V", "Y":
		return "DCE", true
	case "AP", "CF", "CJ", "CY", "FG", "JR", "MA", "OI", "PF", "PK", "PM", "PR", "RM", "RS", "SA", "SF", "SM", "SR", "TA", "UR", "WH", "ZC":
		return "CZCE", true
	case "IC", "IF", "IH", "IM", "T", "TF", "TL", "TS":
		return "CFFEX", true
	default:
		return "", false
	}
}

// isMACNightSession 判断 MAC bar 是否落在需要交易日历转换的期货夜盘时段。
func isMACNightSession(seconds uint32) bool {
	return seconds >= 21*60*60 || seconds <= 5*60*60
}

// futuresPreTradeDate 查询指定期货交易日对应的上一交易日。
func futuresPreTradeDate(db *sql.DB, exchange string, tradeDate uint32) (time.Time, error) {
	var value string
	if err := db.QueryRow(`
		SELECT pretrade_date
		FROM futures_trade_calendar
		WHERE exchange = ? AND cal_date = ? AND is_open = 1`, exchange, fmt.Sprintf("%08d", tradeDate)).Scan(&value); err != nil {
		return time.Time{}, fmt.Errorf("find %s pretrade_date for %08d: %w", exchange, tradeDate, err)
	}
	return parseCalendarDate(value)
}

// combineFuturesDateTime 将日历自然日与 MAC seconds 组合为上海时区时间。
func combineFuturesDateTime(date time.Time, seconds uint32) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), int(seconds/3600), int(seconds%3600/60), int(seconds%60), 0, date.Location())
}

// parseCalendarDate 将 SQLite 中的 YYYYMMDD 日期解析为上海时区起点。
func parseCalendarDate(value string) (time.Time, error) {
	if len(value) != 8 {
		return time.Time{}, fmt.Errorf("invalid calendar date %q", value)
	}
	location, err := time.LoadLocation(futuresCalendarLocation)
	if err != nil {
		return time.Time{}, fmt.Errorf("load %s: %w", futuresCalendarLocation, err)
	}
	year, err := strconv.Atoi(value[:4])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse calendar year %q: %w", value, err)
	}
	month, err := strconv.Atoi(value[4:6])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse calendar month %q: %w", value, err)
	}
	day, err := strconv.Atoi(value[6:])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse calendar day %q: %w", value, err)
	}
	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, location)
	if date.Year() != year || int(date.Month()) != month || date.Day() != day {
		return time.Time{}, fmt.Errorf("invalid calendar date %q", value)
	}
	return date, nil
}
