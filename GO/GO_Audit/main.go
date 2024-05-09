package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/elastic/go-libaudit/v2/auparse"

	"github.com/fsnotify/fsnotify"
)

type EventLog struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

func findFromEnd(filePath, searchString string) (int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}

	fileSize := stat.Size()
	bufferSize := int64(1024) // 定义每次读取的块大小
	var position int64

	// 用于保存从文件读取的临时数据
	buffer := make([]byte, bufferSize)

	// 从文件末尾开始逐块向前读取
	for position = fileSize; position > 0; position -= bufferSize {
		if position < bufferSize {
			buffer = make([]byte, position)
		}

		start := position - bufferSize
		if start < 0 {
			start = 0
		}

		_, err := file.Seek(start, io.SeekStart)
		if err != nil {
			return 0, err
		}

		n, err := file.Read(buffer)
		if err != nil {
			return 0, err
		}

		// 将读取的块转换为字符串并搜索
		bufferStr := string(buffer[:n])
		// 因为读取的块可能会切割行，所以需要适当处理这种情况
		index := reverseFind(bufferStr, searchString)
		if index != -1 {
			// 计算并返回找到字符串的位置
			return start + int64(index), nil
		}
	}

	return 0, fmt.Errorf("string not found")
}

// reverseFind 查找字符串在缓冲区的位置
func reverseFind(buffer, searchString string) int {
	lastIndex := -1
	index := 0
	for {
		index = strings.LastIndex(buffer[index:], searchString)
		if index == -1 {
			break
		}
		lastIndex = index
		index += len(searchString)
	}
	return lastIndex
}

func main() {
	auditPattern := regexp.MustCompile(`msg=audit\(([\d.]+:\d+)\)`)

	filePath := "/var/log/audit/audit.log"
	searchString := "msg=audit(1715161361.788:579)"
	position, err := findFromEnd(filePath, searchString)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Found '%s' at byte position %d\n", searchString, position)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	defer watcher.Close()

	file, err := os.Open("/var/log/audit/audit.log")
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	defer file.Close()

	_, err = file.Seek(position, io.SeekStart)
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}

	err = watcher.Add("/var/log/audit/audit.log")
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	fmt.Println("Now watching /var/log/audit/audit.log...")
	for {
		select {
		case event := <-watcher.Events:

			if event.Op&fsnotify.Write == fsnotify.Write {
				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					line := scanner.Text()
					matches := auditPattern.FindStringSubmatch(line)
					if len(matches) > 1 {
						leastID = matches[1]
						// matches[1] 是括号内匹配的部分
						fmt.Println("Audit record:", matches[1])
					}
					fmt.Println("New log entry:", line)
					handelMessage(line)
				}
				if err := scanner.Err(); err != nil {
					fmt.Println("ERROR:", err)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Println("ERROR:", err)
		}
	}
}

var leastID string
var validID string
var eventList []auparse.AuditMessage

func handelMessage(line string) {
	if leastID == validID {
		auditMsg, err := auparse.ParseLogLine(line)
		if err != nil {
			return
		}
		eventList = append(eventList, *auditMsg)
		return
	}

	if len(eventList) != 0 {

		jsonData, err := json.MarshalIndent(eventList, "", "  ")
		if err != nil {
			fmt.Println("Error marshalling to JSON:", err)
			return
		}
		fmt.Println(string(jsonData))
	}

	hasKeyValue := regexp.MustCompile(`key="([^"]+)"`)

	if matches := hasKeyValue.FindStringSubmatch(line); matches == nil {
		fmt.Println("invalid value:", line)
		return
	}
	validID = leastID
	auditMsg, err := auparse.ParseLogLine(line)
	if err != nil {
		return
	}
	eventList = append([]auparse.AuditMessage{}, *auditMsg)

	// handle eventlist

}
