package main

/*
	pow的时候，不涉及到文件的重新读写，因为所有节点pow之后，都会进入
*/

import (
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

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
		if hashRes[:5] != "00000" {
			nonce++
			continue
		} else {
			// 计算成功，进行下一步脚本
			break
		}
	}
}

/*
	Address
	Description
	ID
	{ublic}


*/
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
	start := time.Now()
	filename := os.Args[1]
	readGroup(filename) // 改成单个节点下的
	in := time.Since(start)
	wg.Wait()
	fmt.Println("pow time consumed: ", in)
}