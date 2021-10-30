package main

/*
	pow的时候，不涉及到文件的重新读写，因为所有节点pow之后，都会进入

	现有记录不靠谱吧，应该用docker起多个节点，然后每个节点都执行这个脚本运行才是
	每个docker都启动一个cothority节点

	现在是在一个机器上，启动多个conode节点。同一台机器同时运行多个节点，是顺序pow的，所以速度会很慢，应该在一台机器上，多个docker同时pow，
	然后同时开启server，再进行后续交易
*/

import (
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
	"go.dedis.ch/onet/v3/app"
)

const MAXINT64 int64 = math.MaxInt64
var lock sync.Mutex
var wg sync.WaitGroup


func HashString(str string) string {
	h := sha256.New()
	h.Write([]byte(str))
	return fmt.Sprintf("%x", h.Sum(nil))
}


func hashCompute(id string, ip string, pk string, des string, wg *sync.WaitGroup) {
	defer wg.Done()
	var nonce int64

	for nonce < MAXINT64 {
		hashRes := HashString(id + ip + string(pk) + des + strconv.FormatInt(nonce, 10))
		if hashRes[:5] != "000" {
			nonce++
			continue
		} else {
			break
		}
	}
}

func readGroup(name string) *app.Group {
	f, err := os.Open(name)
	if err != nil {
		fmt.Println("err open file")
		return nil
	}
	group, err := app.ReadGroupDescToml(f)
	if len(group.Roster.List) == 0 {
		fmt.Println("err open file")
		return nil
	}
	for i := 0; i < len(group.Roster.List); i++ {
		fmt.Println(group.Roster.List[i])
		wg.Add(1)
		go hashCompute(group.Roster.ID.String(), group.Roster.List[0].URL, group.Roster.List[i].Public.String(), group.Roster.List[i].Description, &wg)
	}
	return group
}

func main() {
	// start := time.Now()
	filename := os.Args[1]
	readGroup(filename) // 改成单个节点下的
	// in := time.Since(start).Microseconds()
	wg.Wait()
	// fmt.Println("pow consumed time: ", in)
}