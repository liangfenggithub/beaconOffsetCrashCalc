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
var allLoopCnt int
var loopCnt int
var addrSelect string
var offsetSelect string

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

	offsetSelect = viper.GetString("mac.offsetSelect")
	pingPeriod = viper.GetInt("mac.pingPeriod")
	beaconTimeInterval = viper.GetInt("mac.beaconTimeInterval")
	allLoopCnt = viper.GetInt("test.loopCnt")
	addrSelect = viper.GetString("node.addrSelect")
	if addrSelect == "list" {
		destAddr = viper.GetIntSlice("node.addrList")
	} else if addrSelect == "range" {
		addrRangeMin := viper.GetInt("node.addrRangeMin")
		addrRangeMax := viper.GetInt("node.addrRangeMax")

		for i := addrRangeMin; i < addrRangeMax; i++ {
			destAddr = append(destAddr, i)
		}
	}

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
	if offsetSelect == "fixed" { //固定偏移
		result = addr
	}
	return uint16(result % uint32(pingPeriod)), nil
}
func nodeCrashTest() {

	nowTimeMs := time.Now().UnixMilli()
	lastBeaconTimeS := uint32((nowTimeMs - nowTimeMs%int64(beaconTimeInterval)) / 1000)
	allSomeOffset := make([]int, pingPeriod, pingPeriod) //相同偏移出现次数统计 index是偏移 val是该偏移出现总数
	slotOccupy := make([]int, pingPeriod, pingPeriod)    //时间槽占用统计 index是时间槽占用个数(0-31) val是index出现次数

	//循环执行N次

	for loopCnt = 0; loopCnt < allLoopCnt; loopCnt++ {

		//单次存储结果初始化(构造指定大小为pingPeriod的切片)
		someOffsetResult := make([]map[int]int, pingPeriod, pingPeriod) //数组序号是偏移量，map key是地址，map value也是地址
		someOffsetArr := make([]offsetCount, pingPeriod, pingPeriod)
		var notAppear []int
		// notAppear := make([]int, pingPeriod, pingPeriod)

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

				// 统计出现多次相同的偏移量
				someOffsetArr[len(res)].count++
				if someOffsetArr[len(res)].offset == nil {

					someOffsetArr[len(res)].offset = make([]int, 0)
				}
				someOffsetArr[len(res)].offset = append(someOffsetArr[len(res)].offset, offset)
			} else {
				notAppear = append(notAppear, offset) //记录未出现的偏移量
				log.Printf("偏移量为 %v 未出现\n", offset)
			}
		}

		//单次结果输出
		log.Printf("**************************** beaonTime:%v loopCnt:%v ******************************************\n", lastBeaconTimeS, loopCnt)
		for i, v := range someOffsetArr {
			// if v.count != 0 && i != 1 {
			if v.count != 0 {
				log.Printf("出现 %v 次相同偏移量 %v 个，偏移量分别是：%#v\n", i, v.count, v.offset)
				allSomeOffset[i] = allSomeOffset[i] + v.count
			}
			// } else {
			// 	log.Printf("未出现的偏移量有: %v 个，偏移量分别是：%#v\n", v.count, v.offset)
			// }
		}
		log.Printf("未出现的偏移量有 %v 个，分别是：%#v\n", len(notAppear), notAppear)

		//时间槽占用统计
		slotOccupy[pingPeriod-1-len(notAppear)]++

		//变化beacon时间
		lastBeaconTimeS = lastBeaconTimeS + uint32(beaconTimeInterval/1000)
	}

	//平均结果输出
	// log.Printf("**************************** beaonTime:%v loopCnt:%v ******************************************\n", lastBeaconTimeS, loopCnt)
	log.Printf("***************************************** %v 个终端 间隔周期 %v 秒 循环 %v 次 总体结果输出***************************************************\n", len(destAddr)+1, beaconTimeInterval/1000, loopCnt)

	for i, v := range allSomeOffset {
		if v != 0 {
			// log.Printf("循环 %v 次beacon周期 出现 %v 次相同偏移量共有 %v 个， 平均有: %v 个\n", allLoopCnt, i, v, float32(v)/float32(allLoopCnt))
			log.Printf("循环 %v 次beacon周期 出现 %v 次相同偏移量共有 %v 个\n", allLoopCnt, i, v)
		}
	}

	for i, v := range slotOccupy {
		if v != 0 {
			log.Printf("循环 %v 次 占用 %v 个槽 的次数有 %v 个\n", allLoopCnt, i, v)
		}
	}
}
func main() {

	nodeCrashTest()

}
