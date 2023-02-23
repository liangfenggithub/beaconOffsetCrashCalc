package main

import (
	"bufio"
	"crypto/aes"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/viper"
)

var destAddr []int

// var addrPool []int
var randomSelectNum []int
var fixAddrList []int
var addrPollRangeRange []int
var testAddrSelect string
var pingPeriod int
var beaconTimeInterval int
var allLoopCnt int
var loopCnt int
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
	//地址池构造
	addrPollRangeRange = viper.GetIntSlice("node.addrPollRangeRange")

	// for i := addrPollRangeRange[0]; i < addrPollRangeRange[1]; i++ {
	// 	addrPool = append(addrPool, i)
	// }
	fixAddrList = viper.GetIntSlice("node.fixAddrList")
	randomSelectNum = viper.GetIntSlice("node.randomSelectNum")
	testAddrSelect = viper.GetString("node.testAddrSelect")

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
func IsContain(items []int, item int) bool {
	for _, eachItem := range items {
		if eachItem == item {
			return true
		}
	}
	return false
}
func nodeCrashTest() {

	nowTimeMs := time.Now().UnixMilli()
	lastBeaconTimeS := uint32((nowTimeMs - nowTimeMs%int64(beaconTimeInterval)) / 1000)
	allSomeOffset := make([]int, pingPeriod, pingPeriod) //相同偏移出现次数统计 index是偏移 val是该偏移出现次数总数(非)
	slotOccupy := make([]int, pingPeriod, pingPeriod)    //时间槽占用统计 index是时间槽占用个数(0-31) val是index出现次数

	//循环执行N次

	for loopCnt = 0; loopCnt < allLoopCnt; loopCnt++ {

		//单次存储结果初始化(构造指定大小为pingPeriod的切片)
		someOffsetResult := make([]map[int]int, pingPeriod, pingPeriod) //数组序号是偏移量，map key是地址，map value也是地址
		sameCntArr := make([]offsetCount, 64, 64)                       //数组序号是偏移量出现次数相同的次数，数组长度也就是最大次数暂定为64次
		var notAppear []int
		// notAppear := make([]int, pingPeriod, pingPeriod)

		//测试目标终端生成
		if testAddrSelect == "fixed" {
			destAddr = fixAddrList
		} else if testAddrSelect == "random" {

			//首先生成测试个数
			rand.Seed(time.Now().UnixNano())
			// n := rand.Intn(11) + 20 // 生成20-31之间的随机数
			addrNum := rand.Intn(randomSelectNum[1]-randomSelectNum[0]) + randomSelectNum[0]

			//其次从地址池中生成测试终端
			destAddr = nil
			for i := 0; i < addrNum; i++ {
			RE_MAKE_RANDOM_ADDR:
				addr := rand.Intn(addrPollRangeRange[1]-addrPollRangeRange[0]) + addrPollRangeRange[0]
				if IsContain(destAddr, addr) {
					// time.Sleep(time.Microsecond)
					//出现地址重复，重新生成
					goto RE_MAKE_RANDOM_ADDR
				}
				destAddr = append(destAddr, addr)

			}
		}
		// log.Printf("生成的测试目标终端有 %v 个 分别是\n", len(destAddr))
		// for _, addr := range destAddr {
		// 	log.Printf("0x%x\n", addr)
		// }

		log.Printf("***************************************** 第 %v 轮 开始测试 **********************************************************************\n", loopCnt)
		log.Printf("生成的测试目标终端有 %v 个,每个终端地址及计算得到的偏移量如下:\n", len(destAddr))

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
				log.Printf("终端 %v (0x%x) 在beacon %v秒 偏移量为 %v\n", addr, addr, lastBeaconTimeS, offset)
			}
		}
		// log.Printf("%#v\n", someOffsetResult)
		log.Printf("每个偏移量出现次数如下:\n")
		for offset, res := range someOffsetResult { //遍历所有offset情况
			if res != nil {
				// log.Printf("偏移量为 %v 有 %v 个终端\n", offset, len(res))
				log.Printf("偏移量: %v 出现  %v 次\n", offset, len(res))

				// 统计出现多次相同的偏移量，
				cnt := len(res)         //cnt是相同偏移量出现的次数
				sameCntArr[cnt].count++ //相同次数的总数
				if sameCntArr[cnt].offset == nil {

					sameCntArr[cnt].offset = make([]int, 0)
				}
				sameCntArr[cnt].offset = append(sameCntArr[cnt].offset, offset) //统计相同次数的偏移量
			} else {
				notAppear = append(notAppear, offset) //记录未出现的偏移量
				// log.Printf("偏移量: %v 未出现\n", offset)
			}
		}

		//单次结果输出
		// log.Printf("**************************** beaonTime:%v 循环次数:%v 测试终端个数:%v ******************************************\n", lastBeaconTimeS, loopCnt, len(destAddr))
		log.Printf("***************************************** %v 个终端 偏移量统计结果如下:************************************************************\n", len(destAddr))

		//倒序遍历相同偏移量次数的数组，统计次数总数
		maxCntFindFlag := 0
		for i := len(sameCntArr) - 1; i >= 0; i-- {
			if sameCntArr[i].count != 0 { //该次数有出现
				log.Printf("出现 %v 次相同的有 %v 个偏移量，偏移量分别是：%#v\n", i, sameCntArr[i].count, sameCntArr[i].offset)

				//只记录相同次数最大的次数 ，比如一轮下发出现2次偏移量相同 和3次偏移量相同，那么只记录3轮偏移量相同
				if maxCntFindFlag == 0 {
					maxCntFindFlag = 1
					allSomeOffset[i] = allSomeOffset[i] + 1 //v.count //这里加1是统计出现相同的次数,而不是偏移量
				}
			}
		}
		log.Printf("未出现的偏移量有 %v 个，分别是：%#v\n", len(notAppear), notAppear)

		//时间槽占用统计
		slotOccupy[pingPeriod-len(notAppear)]++

		//变化beacon时间
		lastBeaconTimeS = lastBeaconTimeS + uint32(beaconTimeInterval/1000)
	}

	//平均结果输出
	// log.Printf("**************************** beaonTime:%v loopCnt:%v ******************************************\n", lastBeaconTimeS, loopCnt)
	log.Printf("********************** 从地址池范围 %v-%v中 随机取出 %v-%v个终端,间隔周期 %v 秒,循环 %v 轮计算 总体结果输出******************************\n",
		addrPollRangeRange[0],
		addrPollRangeRange[1],
		randomSelectNum[0],
		randomSelectNum[1],
		beaconTimeInterval/1000, loopCnt)

	for i, v := range allSomeOffset {
		if v != 0 {
			// log.Printf("循环 %v 次beacon周期 出现 %v 次相同偏移量共有 %v 个， 平均有: %v 个\n", allLoopCnt, i, v, float32(v)/float32(allLoopCnt))
			log.Printf("循环 %v 轮beacon周期中 出现 %v 次相同偏移量共有 %v 轮, 理论通信耗时需要 %v 个周期,占总通信轮数%v%% \n", allLoopCnt, i, v, i, float64(v)/float64(allLoopCnt)*100)
		}
	}
	log.Printf("********************** 从地址池范围 %v-%v中 随机取出 %v-%v个终端,间隔周期 %v 秒,循环 %v 轮计算 槽占用统计输出******************************\n",
		addrPollRangeRange[0],
		addrPollRangeRange[1],
		randomSelectNum[0],
		randomSelectNum[1],
		beaconTimeInterval/1000, loopCnt)
	for i, v := range slotOccupy {
		if v != 0 {
			log.Printf("循环 %v 轮 有 %v 轮 占用 %v 个槽\n", allLoopCnt, v, i)
		}
	}
}
func waitUserPushExit() {
	fmt.Println("Press any key to exit...")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
}
func main() {

	nodeCrashTest()
	waitUserPushExit()
}
