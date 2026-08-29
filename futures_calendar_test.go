// futures_calendar_test 验证 MAC 期货 K 线基于交易日历转换自然时间。
package gotdx

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/bensema/gotdx/proto"
	_ "modernc.org/sqlite"
)

// TestResolveMACFuturesDateTimes 验证普通交易日夜盘按上一交易日与次日凌晨转换。
func TestResolveMACFuturesDateTimes(t *testing.T) {
	// 验证 ymd=8 月 27 日对应 8 月 26 日晚和 8 月 27 日凌晨。
	path := newTestFuturesCalendar(t, []testCalendarRow{{"SHFE", "20260827", "20260826"}})
	items := []proto.MACSymbolBar{
		{RawYMD: 20260827, RawSeconds: 21 * 60 * 60},
		{RawYMD: 20260827, RawSeconds: 0},
		{RawYMD: 20260827, RawSeconds: 9 * 60 * 60},
	}

	if err := resolveMACFuturesDateTimes(items, "AG2610", path); err != nil {
		t.Fatalf("resolve MAC futures datetime: %v", err)
	}
	assertDateTime(t, items[0].DateTime, "2026-08-26 21:00:00")
	assertDateTime(t, items[1].DateTime, "2026-08-27 00:00:00")
	if !items[2].DateTime.IsZero() {
		t.Fatalf("day-session datetime was changed: %v", items[2].DateTime)
	}
}

// TestResolveMACFuturesDateTimesWeekend 验证周一交易日可还原周五晚和周六凌晨。
func TestResolveMACFuturesDateTimesWeekend(t *testing.T) {
	// 验证 ymd=8 月 24 日通过 pretrade_date=8 月 21 日处理跨周末夜盘。
	path := newTestFuturesCalendar(t, []testCalendarRow{{"SHFE", "20260824", "20260821"}})
	items := []proto.MACSymbolBar{
		{RawYMD: 20260824, RawSeconds: 21*60*60 + 30*60},
		{RawYMD: 20260824, RawSeconds: 2 * 60 * 60},
	}

	if err := resolveMACFuturesDateTimes(items, "AG2610", path); err != nil {
		t.Fatalf("resolve MAC futures datetime: %v", err)
	}
	assertDateTime(t, items[0].DateTime, "2026-08-21 21:30:00")
	assertDateTime(t, items[1].DateTime, "2026-08-22 02:00:00")
}

// TestFuturesExchange 验证主要期货交易所的合约前缀映射。
func TestFuturesExchange(t *testing.T) {
	// 验证各交易所常用合约可正确映射到 Tushare 交易所代码。
	cases := map[string]string{"AG2610": "SHFE", "SC2610": "INE", "I2609": "DCE", "SR609": "CZCE", "IF2609": "CFFEX"}
	for code, expected := range cases {
		actual, ok := futuresExchange(code)
		if !ok || actual != expected {
			t.Fatalf("futures exchange for %s = %q, %t; want %q, true", code, actual, ok, expected)
		}
	}
}

// testCalendarRow 表示测试 SQLite 中的一条开市日记录。
type testCalendarRow struct {
	exchange      string
	calendarDate  string
	preTradingDay string
}

// newTestFuturesCalendar 创建只包含测试交易日的 SQLite 日历文件。
func newTestFuturesCalendar(t *testing.T, rows []testCalendarRow) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "calendar.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open test calendar: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE futures_trade_calendar (
			exchange TEXT NOT NULL,
			cal_date TEXT NOT NULL,
			is_open INTEGER NOT NULL,
			pretrade_date TEXT NOT NULL,
			synced_at_unix INTEGER NOT NULL,
			PRIMARY KEY (exchange, cal_date)
		)`); err != nil {
		t.Fatalf("create test calendar: %v", err)
	}
	for _, row := range rows {
		if _, err := db.Exec(`INSERT INTO futures_trade_calendar VALUES (?, ?, 1, ?, 0)`, row.exchange, row.calendarDate, row.preTradingDay); err != nil {
			t.Fatalf("insert test calendar: %v", err)
		}
	}
	return path
}

// assertDateTime 验证上海时区时间与预期自然时间一致。
func assertDateTime(t *testing.T, actual time.Time, expected string) {
	t.Helper()
	if actual.Format(time.DateTime) != expected {
		t.Fatalf("datetime = %s; want %s", actual.Format(time.DateTime), expected)
	}
}
