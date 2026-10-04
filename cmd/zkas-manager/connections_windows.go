package main

import (
	"encoding/binary"
	"fmt"
	"golang.org/x/sys/windows"
	"unsafe"
)

// The bridge binds IPv4. Count established TCP sessions separately from workers
// with recent shares: a socket can be connected before a worker submits a share.
func stratumConnections() (int, error) {
	proc := windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetTcpTable")
	size := uint32(4096)
	for attempt := 0; attempt < 4; attempt++ {
		if size < 4 || size > 32<<20 {
			return 0, fmt.Errorf("invalid TCP table size")
		}
		data := make([]byte, size)
		code, _, _ := proc.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&size)), 0)
		if code == 122 {
			continue
		}
		if code != 0 {
			return 0, fmt.Errorf("TCP table error %d", code)
		}
		rows := binary.LittleEndian.Uint32(data[:4])
		if uint64(rows)*20+4 > uint64(len(data)) {
			return 0, fmt.Errorf("truncated TCP table")
		}
		count := 0
		for i := uint32(0); i < rows; i++ {
			row := data[4+i*20 : 4+(i+1)*20]
			if binary.LittleEndian.Uint32(row[:4]) == 5 && binary.BigEndian.Uint16(row[8:10]) == 5555 {
				count++
			}
		}
		return count, nil
	}
	return 0, fmt.Errorf("TCP table changed; retry")
}
