package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

func sanitizeKey(key string) string {
	return strings.Replace(key, ".", "_", -1)
}

func zabbixResponse(processed, failed, total int, seconds float64) []byte {
	responseString := fmt.Sprintf(`{"response": "success", "info": "processed: %d; failed: %d; total: %d; seconds spent: %f"}`,
		processed, failed, total, seconds)

	size := make([]byte, 8)
	binary.LittleEndian.PutUint64(size, uint64(len(responseString)))

	buf := bytes.NewBuffer([]byte("ZBXD\x01"))
	buf.Write(size)
	buf.WriteString(responseString)

	return buf.Bytes()
}
