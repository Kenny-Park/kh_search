package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"kh_search"
	"os"
	"runtime"
	"time"
)

// 메인함수
func main() {

	fmt.Println(runtime.NumCPU())
	runtime.GOMAXPROCS(8)
	file, err := os.Open(`./data.txt`)

	// csv reader 생성
	rdr := csv.NewReader(bufio.NewReader(file))

	// csv 내용 모두 읽기
	rows, err := rdr.ReadAll()
	if err != nil {
		panic(err)
	}

	var raws []string

	fmt.Println("rows", len(rows))

	// 행,열 읽기
	for i, row := range rows {
		for j := range row {
			raws = append(raws, rows[i][j])
		}
	}

	f := &kh_search.KhSearch{
		Raws: raws,
	}

	// 트라이 트리 만들기
	f.MakeTree()

	startTime := time.Now()

	fmt.Println(f.Search("신세계"))
	fmt.Println(f.Search("가"))

	elapsedTime := time.Since(startTime)

	fmt.Printf("실행시간: %s\n", elapsedTime)

}
