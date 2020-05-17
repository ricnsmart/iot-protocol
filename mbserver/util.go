package mbserver

import (
	"fmt"
	"strings"
	"time"
)

/*
将物联网设备中常见的形如20, 3, 12, 17, 19, 00的字节流，转换为形如2020-03-12 17:19:00这样的格式
*/
func BytesDecodeTime(packet []byte) string {
	// 边界检查
	_ = packet[5]
	var b strings.Builder
	year := packet[0]
	month := packet[1]
	day := packet[2]
	hour := packet[3]
	minute := packet[4]
	second := packet[5]
	b.WriteString(time.Now().Format("2006")[0:2])
	b.WriteString(fmt.Sprintf(`%v-`, year))
	if month >= 10 {
		b.WriteString(fmt.Sprintf(`%v-`, month))
	} else {
		b.WriteString(fmt.Sprintf(`0%v-`, month))
	}
	if day >= 10 {
		b.WriteString(fmt.Sprintf(`%v `, day))
	} else {
		b.WriteString(fmt.Sprintf(`0%v `, day))
	}
	if hour >= 10 {
		b.WriteString(fmt.Sprintf(`%v:`, hour))
	} else {
		b.WriteString(fmt.Sprintf(`0%v:`, hour))
	}
	if minute >= 10 {
		b.WriteString(fmt.Sprintf(`%v:`, minute))
	} else {
		b.WriteString(fmt.Sprintf(`0%v:`, minute))
	}
	if second >= 10 {
		b.WriteString(fmt.Sprintf(`%v`, second))
	} else {
		b.WriteString(fmt.Sprintf(`0%v`, second))
	}
	return b.String()
}
