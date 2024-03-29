package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type Result struct {
	SearchWord string
	Values     []string
}

type Edge struct {
	Value       int
	EOF         bool
	Left        map[int]*Edge   // child
	LeftList    []*Edge         // 모든노드 검색
	Right       *Edge           // header
	ChosungLeft map[int][]*Edge // 초성용 child
	Fl          *Edge           //failure link
	Visited     map[string]bool
}

type ChannelObj struct {
	MergeChan chan []string
	MergeDone chan []string
}

type KhSearch struct {
	Raws  []string
	Bfs   map[int][]*Edge
	Root  *Edge
	mutex sync.Mutex
}

type ResultValue struct {
	results []string
	m       sync.Mutex
}

func (r *ResultValue) GetResults() []string {
	r.m.Lock()
	defer r.m.Unlock()
	return r.results
}
func (r *ResultValue) Append(val []string) {
	r.m.Lock()
	defer r.m.Unlock()
	r.results = append(r.results, val...)
	log.Println(r.results)
}

var mutex = &sync.RWMutex{}

//초성,중성,종성 세팅
var CHOUNICODE []int = []int{0x3131, 0x3132, 0x3134, 0x3137, 0x3138, 0x3139, 0x3141, 0x3142, 0x3143, 0x3145, 0x3146,
	0x3147, 0x3148, 0x3149, 0x314a, 0x314b, 0x314c, 0x314d, 0x314e}
var JUNGUNICODE []int = []int{0x314f, 0x3150, 0x3151, 0x3152, 0x3153, 0x3154, 0x3155, 0x3156, 0x3157, 0x3158, 0x3159,
	0x315a, 0x315b, 0x315c, 0x315d, 0x315e, 0x315f, 0x3160, 0x3161, 0x3162, 0x3163}
var JONGUNICODE []int = []int{0x0000, 0x3131, 0x3132, 0x3133, 0x3134, 0x3135, 0x3136, 0x3137, 0x3139, 0x313a, 0x313b,
	0x313c, 0x313d, 0x313e, 0x313f, 0x3140, 0x3141, 0x3142, 0x3144, 0x3145, 0x3146, 0x3147, 0x3148,
	0x314a, 0x314b, 0x314c, 0x314d, 0x314e}

// 트리를 만든다.
func (f *KhSearch) MakeTree() *Edge {

	f.Root = f.addEdge(nil, -1, 0, false, false, nil)
	edge := f.Root
	for _, item := range f.Raws {
		words := GetWordAtRows(item)
		var jamos []int
		for _, word := range words {
			jamos = append(jamos, f.getJamo(word)...)
		}
		for d, jamo := range jamos {
			isChosung := false
			if d%3 == 0 && d > 0 {
				isChosung = true
			}
			if d == len(jamos)-1 {
				edge = f.addEdge(edge, jamo, d, true, isChosung, jamos)
			} else {
				edge = f.addEdge(edge, jamo, d, false, isChosung, jamos)
			}
		}
		edge = f.Root
	}
	return edge
}

// 키워드를 아스키로 변경
func (f *KhSearch) ascii(keyword string) int {
	length := len(keyword)
	if length <= 0 {
		return 0
	}
	hex := int(keyword[0])
	if hex <= 0x7F {
		return hex
	}
	if hex < 0xC2 {
		return 0
	}
	if hex <= 0xDF && length > 1 {
		return (hex&0x1F)<<6 | (int(keyword[1]) & 0x3F)
	}
	if hex <= 0xEF && length > 2 {
		return (hex&0x0F)<<12 | (int(keyword[1])&0x3F)<<6 | (int(keyword[2]) & 0x3F)
	}
	if hex <= 0xF4 && length > 3 {
		return (hex&0x0F)<<18 | (int(keyword[1])&0x3F)<<12 | (int(keyword[2])&0x3F)<<6 | (int(keyword[3]) & 0x3F)
	}
	return 0
}

func (f *KhSearch) AddNewList(v string) {
	edge := f.Root
	words := GetWordAtRows(v)
	var jamos []int
	for _, word := range words {
		jamos = append(jamos, f.getJamo(word)...)
	}
	for d, jamo := range jamos {
		isChosung := false
		if d%3 == 0 && d > 0 {
			isChosung = true
		}
		if d == len(jamos)-1 {
			edge = f.addEdge(edge, jamo, d, true, isChosung, jamos)
		} else {
			edge = f.addEdge(edge, jamo, d, false, isChosung, jamos)
		}
	}
}

// 한글자를 자모 배열로 변경
func (f *KhSearch) getJamo(word string) []int {
	var jamoArrray []int
	//a := []rune(word)
	//s := fmt.Sprintf("0x%x", word)

	s2 := f.ascii(word)

	if s2 >= 44032 && s2 < 55203 {
		chosung := (((s2 - 0xAC00) - (s2-0xAC00)%28) / 28) / 21
		jamoArrray = append(jamoArrray, CHOUNICODE[chosung])
		jungsung := (((s2 - 0xAC00) - (s2-0xAC00)%28) / 28) % 21
		jamoArrray = append(jamoArrray, JUNGUNICODE[jungsung])
		jongsung := (s2 - 0xAC00) % 28
		jamoArrray = append(jamoArrray, JONGUNICODE[jongsung])
	} else {
		jamoArrray = append(jamoArrray, []int{s2, 0, 0}...)
	}
	return jamoArrray
}

// 한글자씩 자른다.
func GetWordAtRows(item string) []string {
	var result []string
	var paramByte []byte

	paramByte = []byte(item)

	idx := 0
	for i := 0; i < len(item); i++ {
		if idx >= len(item) {
			break
		}
		s, sz := utf8.DecodeRune(paramByte[idx:])
		result = append(result, string(s))

		idx += sz
	}
	return result
}

// 초성 전용 트리 구성
func (f *KhSearch) findChosungEdge(t *Edge, word int) {
	// 헤더를 찾는다
	findedEdge := t.Right.Right.Right
	findedEdgeLeft, ok := findedEdge.ChosungLeft[word]

	if ok {
		for _, item := range findedEdgeLeft {
			if item == t {
				return
			}
		}
		findedEdgeLeft = append(findedEdgeLeft, t)
		findedEdge.ChosungLeft[word] = findedEdgeLeft
	} else {
		var tmp []*Edge
		tmp = append(tmp, t)
		findedEdge.ChosungLeft[word] = tmp

	}
}

// 실패 링크 설정
func (f *KhSearch) setFailure(t *Edge, word int) *Edge {
	if t.Right == nil || t.Right.Value == -1 {
		return nil
	}
	right := t.Right
	t, ok := right.Left[word]

	if !ok {
		return f.setFailure(right.Right, word)
	} else {
		return t
	}
}

// 엣지를 추가한다.
func (f *KhSearch) addEdge(t *Edge, word int, depth int, EOF bool, isChosung bool, jamos []int) *Edge {
	// header
	if t == nil {
		return &Edge{
			Value: -1,
			Left:  make(map[int]*Edge),
		}
	}
	// non header
	var r *Edge

	if _, ok := t.Left[word]; !ok {
		r = &Edge{
			Value:       word,
			Left:        make(map[int]*Edge),
			Right:       t,
			EOF:         EOF,
			ChosungLeft: make(map[int][]*Edge),
			Fl:          f.setFailure(t, word),
		}
		t.Left[word] = r
		t.LeftList = append(t.LeftList, r)

		tmp := f.Bfs[word]
		f.Bfs[word] = append(tmp, r)

	} else {
		r = t.Left[word]
	}
	if isChosung {
		f.findChosungEdge(r, word)
	}
	return r
}

// 전채를 검색한다.
func (f *KhSearch) forestAllSearch(t *Edge, result []string, pre []int, jasaw []int, isFirst bool) []string {

	if t == nil {
		return result
	}
	if !isFirst {
		jasaw = append(jasaw, t.Value)
		if t.EOF {
			s := f.jamosToString(append(pre, jasaw...))
			result = append(result, s)
		}
	}
	if len(t.LeftList) > 0 {
		for _, key := range t.LeftList {
			if obj, ok := t.Left[key.Value]; ok {
				result = f.forestAllSearch(obj, result, pre, jasaw, false)
			} else {
				result = f.forestAllSearch(nil, result, pre, jasaw, false)
			}
		}
	}
	result = f.forestAllSearch(nil, result, pre, jasaw, false)
	return result
}

// 이전 문자 검색
func (f *KhSearch) preString(edge *Edge, jasaw []int) []int {
	if edge == nil {
		var tmp []int
		for i := len(jasaw) - 1; i >= 0; i-- {
			tmp = append(tmp, jasaw[i])
		}
		return tmp
	}
	jasaw = append(jasaw, edge.Value)
	return f.preString(edge.Right, jasaw)

}

// 너비 검색
func (f *KhSearch) Search(word string) []string {
	var words []string
	idx := 0
	var paramByte []byte
	var resultStrings []string

	log.Println("words :", word)

	paramByte = []byte(word)

	for i := 0; i < len(word); i++ {
		if idx >= len(word) {
			break
		}
		s, sz := utf8.DecodeRune(paramByte[idx:])
		words = append(words, string(s))
		idx += sz
	}
	var jamos []int
	var jasaws []int

	for _, jamo := range words {
		jamos = append(jamos, f.getJamo(jamo)...)
	}

	jasaws = append(jasaws, jamos[0])

	chosungSearchOper := true

	rv := &ResultValue{}

	//fmt.Println(jamos)

	lastChosung := jamos[len(jamos)-3 : len(jamos)]

	for i, item := range jamos {
		if (i%3 == 1 || i%3 == 2) && item != 0 {
			chosungSearchOper = false
		}
	}
	if !chosungSearchOper && len(lastChosung) == 3 && lastChosung[1] == 0 && lastChosung[2] == 0 {
		jamos = jamos[:len(jamos)-2]
	}
	var threadCheck = 4

	edges, ok := f.Bfs[jamos[0]]
	if chosungSearchOper {
		if len(jamos) < 4 {
			return nil
		}
	}

	wg := sync.WaitGroup{}
	// 쓰레드를 다수 사용할 경우
	if ok {
		if len(edges)/threadCheck > 20 {
			for i := 1; i <= threadCheck; i++ {
				wg.Add(1)
				a := i * (len(edges) / threadCheck)
				if i*(len(edges)/threadCheck) > len(edges) {
					a = len(edges)
				}
				go f.searchExecutor(edges[(i-1)*(len(edges)/threadCheck):a],
					jamos,
					chosungSearchOper,
					jasaws,
					&wg,
					rv,
				)
			}
		} else { // 쓰레드가 1개일 경우
			wg.Add(1)
			threadCheck = 1
			go f.searchExecutor(edges, jamos, chosungSearchOper, jasaws, &wg, rv)
		}
	} else {
		return nil
	}
	wg.Wait()
	dupCheck := make(map[string]bool)
	mergedResults := rv.GetResults()
	for _, item := range mergedResults {
		if _, ok := dupCheck[item]; !ok {
			resultStrings = append(resultStrings, item)
			dupCheck[item] = true
		}
	}
	return resultStrings
}

func (f *KhSearch) mergeCheck(threadCheck int, channelObj *ChannelObj) {
	var resultStrings []string
	mergeChanCheck := 0
	for {
		select {
		case r := <-channelObj.MergeChan:
			mergeChanCheck++
			resultStrings = append(resultStrings, r...)
			if mergeChanCheck >= threadCheck {
				channelObj.MergeDone <- resultStrings
				break
			}
		}
	}
}

// 추후 go routine을 이용한 검색으로 변경
func (f *KhSearch) searchExecutor(edges []*Edge, jamos []int, chosungSearchOper bool, jasaws []int, wg *sync.WaitGroup, result *ResultValue) {
	var resultStrings []string
	defer wg.Done()
	for _, edge := range edges {
		if chosungSearchOper {
			reChosung := f.chosungverticalSearch(edge, jamos, 3, nil, nil)
			if len(reChosung) > 0 {
				for _, item := range reChosung {
					resultStrings = append(resultStrings, item)
				}
			}
		} else {
			re := f.verticalSearch(edge, jamos, 1, nil, jasaws)

			if len(re) > 0 {
				pre := f.preString(edge.Right, nil)
				for _, item := range re {
					resultStrings = append(resultStrings, f.jamosToString(pre[1:])+item)
				}
			}
		}
	}
	result.Append(resultStrings)
}

// UTF-8 -> string 변환
func (f *KhSearch) jamosToString(item2 []int) string {
	ifix := 0
	i0 := -1
	i1 := -1
	i2 := -1
	resultString := ""
	if len(item2) < 3 {
		return ""
	}
	for j := 0; j < len(item2); j++ {
		if ifix != int(j/3) {
			jamoSum := 0
			if i1 >= 0 {
				jamoSum = 0xAC00 + 28*21*i0 + 28*i1 + i2
			} else {

				jamoSum = item2[j-3]
			}

			jamoSumFormat := fmt.Sprintf("%q", jamoSum)
			resultString += strings.Trim(jamoSumFormat, "'")
			ifix = int(j / 3)
			i0 = -1
			i1 = -1
			i2 = -1
		}
		if j%3 == 0 {
			for d, val := range CHOUNICODE {
				if val == item2[j] {
					i0 = d
				}
			}
		} else if j%3 == 1 {
			for d, val := range JUNGUNICODE {
				if val == item2[j] {
					i1 = d
				}
			}
		} else {
			for d, val := range JONGUNICODE {
				if val == item2[j] {
					i2 = d
				}
			}
		}
	}
	jamoSum := 0
	jamoSumFormat := ""
	if i1 >= 0 {
		jamoSum = 0xAC00 + 28*21*i0 + 28*i1 + i2
		jamoSumFormat = fmt.Sprintf("%q", jamoSum)
	} else {
		jamoSum = item2[len(item2)-3]
		jamoSumFormat = fmt.Sprintf("%q", jamoSum)
	}

	resultString += strings.Trim(jamoSumFormat, "'")
	return resultString
}

// 깊이 검색
func (f *KhSearch) verticalSearch(edge *Edge, words []int, index int, results []string, jasaw []int) []string {

	if edge == nil || index < 0 {
		return results
	}
	a, ok := edge.Left[words[index]]
	if ok {
		index++
		jasaw = append(jasaw, a.Value)
		if index >= len(words) {
			if a.EOF {
				results = append(results, f.jamosToString(jasaw))
			}
			return f.forestAllSearch(a, results, jasaw, nil, true)
		}
		return f.verticalSearch(a, words, index, results, jasaw)
	}
	return f.verticalSearch(nil, words, index, results, jasaw)

}

// FailureLink 체크
func (f *KhSearch) failureCheck(t *Edge, jasaw []int) ([]int, bool) {
	p := f.preString(t.Fl, nil)
	pLength := len(p)
	jasawLength := len(jasaw)
	if pLength >= jasawLength {
		p = p[len(p)-len(jasaw):]
		if reflect.DeepEqual(jasaw, p) && len(p) > 1 {
			return p, true
		}
	}
	return jasaw, false
}

// 초성 검색
func (f *KhSearch) chosungverticalSearch(edge *Edge, words []int, index int, results []string, jasaw []int) []string {
	if edge == nil {
		return results
	}

	if index < 0 || index >= len(words) {
		return results
	}

	a, ok := edge.ChosungLeft[words[index]]
	index += 3

	if ok {
		for i := 0; i < len(a); i++ {
			if index >= len(words) {
				results = f.forestAllSearch(a[i], results, f.preString(a[i], nil)[1:], nil, true)
			}
			results = f.chosungverticalSearch(a[i], words, index, results, nil)
		}
	}
	results = f.chosungverticalSearch(nil, words, index, results, nil)
	return results
}

// 메인함수
func main() {
	runtime.GOMAXPROCS(4)
	rp, _ := os.Getwd()

	var rows [][]string
	func() {
		file, err := os.Open(rp + `/data.txt`)
		defer file.Close()

		// csv reader 생성
		rdr := csv.NewReader(bufio.NewReader(file))

		// csv 내용 모두 읽기
		rows, err = rdr.ReadAll()
		if err != nil {
			panic(err)
		}
	}()

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
	go func() {
		t := time.NewTicker(time.Minute)
		for {
			select {
			case <-t.C:

				log.Println("새로운 데이타를 가져옵니다.")
				file2, err := os.Open(rp + `/data_new.txt`)
				if err != nil {
					log.Println("파일이 없습니다.")
					break
				}

				// csv reader 생성
				rdr := csv.NewReader(bufio.NewReader(file2))

				// csv 내용 모두 읽기
				rows, err = rdr.ReadAll()
				if err != nil {
					panic(err)
				}
				file2.Close()

				for i, row := range rows {
					for j := range row {
						raws = append(raws, rows[i][j])
					}
				}

				for _, item := range raws {
					f.AddNewList(item)
				}

				os.Remove(rp + `/data_new.txt`)
				break
			}
		}
	}()

	http.HandleFunc("/search", func(w http.ResponseWriter, req *http.Request) {
		startTime := time.Now()
		sw := req.URL.Query().Get("word")

		r := &Result{
			SearchWord: sw,
			Values:     f.Search(sw),
		}

		rbyte, _ := json.Marshal(r)
		elapsedTime := time.Since(startTime)
		fmt.Printf("실행시간: %s\n", elapsedTime)
		w.Write(rbyte)
	})
	http.ListenAndServe(":5000", nil)
}
