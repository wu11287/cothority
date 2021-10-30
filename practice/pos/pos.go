// POS + VRF 方法实现
/*
	每个节点有ip、description、id、pk，基本信息
	每个节点有自己的初始化权重信息，（默认是1） ---  权重也只在开始的时候有利

	客户端不需要实现，只需要实现节点那部分就可以
	每个节点
		1. 生成公私钥
		2.
*/

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	// "fmt"
	crypto "go_grammar/practice/crypto"
	initial "go_grammar/practice/initial"
	"log"
	"sync"
	// "time"
)

var lock sync.Mutex
const denominator float64 = 18446744073709551615

func main() {
	var wg sync.WaitGroup
	//生成随机值
	randomness, err := initial.GenerateRandomBytes(10) //不用seed会产生确定性结果,作为初始状态下传递的消息
	if err != nil {
		log.Fatalf("generate randomness error: %v", err)
	}
	// var pk crypto.VrfPubkey 
	// var data []byte = []byte(pk_str)
	// pk = crypto.VrfPubkey(data)
	// var sk crypto.VrfPrivkey 
	// pk, sk := crypto.VrfKeygen() // 每次 VrfKeygen 运行的结果都不一样
	// ok := Sortition(randomness, &wg, pk, sk)
	// log.Printf("ok = %v\n", ok)
	start := time.Now()

	// for i:=0; i<10; i++ {
	pk, sk := crypto.VrfKeygen() // 每次 VrfKeygen 运行的结果都不一样
	ok := Sortition(randomness, &wg, pk, sk)
	log.Printf("ok = %v\n", ok)
	// }

	interval := time.Since(start).Microseconds()
	fmt.Printf("time consumed: %v μs\n", interval)
}

func Sortition(msg []byte, wg *sync.WaitGroup, pk crypto.VrfPubkey, sk crypto.VrfPrivkey) bool {
	lock.Lock()

	proof, ok := sk.ProveMy(msg)
	if !ok {
		log.Fatal("generate proof error")
	}

	rnd, ok := proof.Hash()
	if !ok {
		log.Fatal("generate rnd error")
	}

	ok = VerifyRnd(rnd)
	lock.Unlock()

	return ok
}


// 判断该节点是否被选中
func VerifyRnd(rnd crypto.VrfOutput) bool {
	bytesBuffer := bytes.NewBuffer(rnd[:])
	var x int64
	binary.Read(bytesBuffer, binary.BigEndian, &x)

	rnd_res := float64(x)
	if rnd_res < 0 {
		rnd_res += denominator
	}

	p := rnd_res / denominator //得到一个概率值
	log.Println("sortition success, I am in system")

	return p < 0.7
}


// 传播的时候没有传播rnd
func VerifyProof(Pk crypto.VrfPubkey, proof crypto.VrfProof, msg []byte) bool {
	ok, _:= Pk.VerifyMy(proof, msg)

	if !ok {
		log.Println("verified error")
		return false
	}
	return ok
}