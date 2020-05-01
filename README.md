# kh_search
golang을 이용한 검색엔진 

kh_search의 특징
1. 멀티쓰레드를 이용한 검색엔진
2. 싱글쓰레드에 비해 약 4~5배 정도 빠른 검색속도 (0ms ~ 170ms, 평균 30ms 내외)
3. 간편한 사용방법


예제

  // 트리 생성
  
	runtime.GOMAXPROCS(4)

	file, err := os.Open(`./kr_korean.txt`)

	// csv reader 생성
	rdr := csv.NewReader(bufio.NewReader(file))

	// csv 내용 모두 읽기
	rows, err := rdr.ReadAll()
	if err != nil {
		panic(err)
	}

	var raws []string
	// 행,열 읽기
	for i, row := range rows {
		for j := range row {
			raws = append(raws, rows[i][j])
		}
	}

	f := &KhSearch{
		Raws: raws,
		Bfs:  make(map[int][]*Edge),
	}
	// 트라이 트리 만들기
	f.MakeTree()

 // 테스트용 웹서버
 
	http.HandleFunc("/search", func(w http.ResponseWriter, req *http.Request) {
		startTime := time.Now()
		sw := req.URL.Query().Get("word")

		r := &Result{
			SearchWord: sw,
			// 검색
      Values:     f.Search(sw),
		}
		rbyte, _ := json.Marshal(r)
		elapsedTime := time.Since(startTime)
		fmt.Printf("실행시간: %s\n", elapsedTime)
		w.Write(rbyte)
	})
  
	http.ListenAndServe(":5000", nil)
