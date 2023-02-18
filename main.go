package main

import (
	"crypto/aes"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/spf13/viper"
)

var destAddr []int
var pingPeriod int
var beaconTimeInterval int
var loopCnt int

type offsetCount struct {
	count  int
	offset []int
}

const CLASSB_BEACON_INTERVAL = 64000 //ms

func init() {
	/*1. 配置文件读取*/
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("./config/")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println("read config failed: %v", err)
		return
	}

	/*2. log文件设置*/
	// logFile, err := os.OpenFile("./run.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	//打开文件
	logFile, err := os.OpenFile(viper.GetString("log.path"), os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		fmt.Println("open log file failed, err:", err)
		return
	}

	/*log包可以通过SetOutput()方法指定日志输出的方式（Writer），
	但是只能指定一个输出的方式（Writer）。
	我们利用io.MultiWriter()将多个Writer拼成一个Writer使用的特性，
	把log.Println()输出的内容分流到控制台和文件当中。*/
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
	log.SetFlags(log.Lmicroseconds | log.Ldate)

	//参数初始化

	destAddr = viper.GetIntSlice("node.addrList")
	pingPeriod = viper.GetInt("mac.pingPeriod")
	beaconTimeInterval = viper.GetInt("mac.beaconTimeInterval")
	loopCnt = viper.GetInt("test.loopCnt")

}

//计算指定终端的偏移
func ComputePingOffset(beaconTime uint32, addr uint32, pingPeriod int) (uint16, error) {
	key := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	input := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var result uint32 = 0

	input[0] = byte(beaconTime)
	input[1] = byte(beaconTime >> 8)
	input[2] = byte(beaconTime >> 16)
	input[3] = byte(beaconTime >> 24)
	input[4] = byte(addr)
	input[5] = byte(addr >> 8)
	input[6] = byte(addr >> 16)
	input[7] = byte(addr >> 24)

	cipher, err := aes.NewCipher(key[:]) //可以使用切片表达式 s[:] 来将数组 arr 转换为切片 s
	if err != nil {
		return 0, err
	}
	output := make([]byte, len(input))
	cipher.Encrypt(output, input[:])

	result = ((uint32(output[0])) + (uint32(output[1]) * 256))
	//偏移方式选择
	return uint16(result % uint32(pingPeriod)), nil
}
func nodeCrashTest() {

	allSomeOffset := make([]int, pingPeriod, pingPeriod)

	nowTimeMs := time.Now().UnixMilli()
	lastBeaconTimeS := uint32((nowTimeMs - nowTimeMs%int64(beaconTimeInterval)) / 1000)

	//循环执行N次

	for loopCnt := 0; loopCnt < 10; loopCnt++ {

		//单次存储结果初始化(构造指定大小为pingPeriod的切片)
		someOffsetResult := make([]map[int]int, pingPeriod, pingPeriod) //int(addr) int (addr)
		someOffsetCount := make([]offsetCount, pingPeriod, pingPeriod)

		log.Printf("*************************************** 第 %v 次 测试结果 *****************************************************\n", loopCnt)
		// 计算所有终端的偏移量
		for _, addr := range destAddr {
			offset, err := ComputePingOffset(lastBeaconTimeS, uint32(addr), pingPeriod)

			if err != nil {
				panic("error")
			} else {
				if someOffsetResult[offset] == nil {
					someOffsetResult[offset] = make(map[int]int)
				}
				someOffsetResult[offset][addr] = addr
				log.Printf("终端 %v 在beacon %v秒 偏移量为 %v\n", addr, lastBeaconTimeS, offset)
			}
		}
		// log.Printf("%#v\n", someOffsetResult)

		for offset, res := range someOffsetResult {
			if res != nil {
				log.Printf("偏移量为 %v 有 %v 个终端\n", offset, len(res))

				// 统计出现 多 次相同偏移量的终端个数
				someOffsetCount[len(res)].count++
				if someOffsetCount[len(res)].offset == nil {

					someOffsetCount[len(res)].offset = make([]int, 0)
				}
				someOffsetCount[len(res)].offset = append(someOffsetCount[len(res)].offset, offset)
			}
		}

		//单次结果输出
		log.Printf("**************************** beaonTime:%v loopCnt:%v ******************************************\n", lastBeaconTimeS, loopCnt)
		for i, v := range someOffsetCount {
			// if v.count != 0 && i != 1 {
			if v.count != 0 {
				log.Printf("出现 %v 次偏移量的终端有: %v 个，偏移量分别是：%#v\n", i, v.count, v.offset)
				allSomeOffset[i] = allSomeOffset[i] + v.count
			}
			// } else {
			// 	log.Printf("未出现的偏移量有: %v 个，偏移量分别是：%#v\n", v.count, v.offset)
			// }
		}
		lastBeaconTimeS = lastBeaconTimeS + uint32(beaconTimeInterval/1000)
	}

	//平均结果输出
	// log.Printf("**************************** beaonTime:%v loopCnt:%v ******************************************\n", lastBeaconTimeS, loopCnt)
	log.Printf("********************************************************************************************\n")

	for i, v := range allSomeOffset {
		if v != 0 {

			log.Printf("%v 的beacon周期 出现 %v 次相同偏移量的终端总个数有 %v 个， 平均有: %v 个\n", loopCnt, i, v, v/loopCnt)
		}
	}
}
func main() {

	nodeCrashTest()

}
