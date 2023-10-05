package main

import (
	"log"
	"time"
	"unsafe"
)

// 常量
const (
	AUTO_COLLECT_ADDR = 0x004352f2
	AUTO_COLLECT_ON   = 0xeb
	AUTO_COLLECT_OFF  = 0x75
)

// @title: pvzWindow
// @description: pvz窗口结构体
type pvzWindow struct {
	// 窗口句柄
	Handle HANDLE
	// 进程ID
	Pid DWORD
	// 进程句柄
	ProcessHandle HANDLE
	// 内存锁
	memoryLock chan struct{}
	// 自动点冰
	autoIceShroom bool
}

// @title: pvzWindow::isValid
// @description: 判断窗口是否有效
// @return: bool
func (pvz *pvzWindow) isValid() bool {
	if pvz.Handle == 0 {
		return false
	}
	exit_code := new(DWORD)
	GetExitCodeProcess(pvz.ProcessHandle, exit_code)
	if *exit_code == 259 {
		return true
	} else {
		return false
	}
}

// @title: pvzWindow::ReadMemory
// @description: 读取内存
// @param: readSize int 读取字节数
// @param: address ...int 内存地址(可以多级偏移)
// @return: interface{}
func (pvz *pvzWindow) ReadMemory(readSize int, address ...int) interface{} {
	if !pvz.isValid() {
		log.Panic("窗口无效!")
	}

	// 加锁
	pvz.memoryLock <- struct{}{}
	defer func() {
		// 解锁
		<-pvz.memoryLock
	}()

	level := len(address)       // 偏移级数
	var offset LPVOID = 0       // 内存地址
	var buffer = new(LPVOID)    // 缓冲区
	var bytesRead = new(SIZE_T) // 读取字节数

	for i := 0; i < level; i++ {
		offset = *buffer + LPVOID(address[i])
		if i != level-1 {
			size := 4
			success := ReadProcessMemory(pvz.ProcessHandle, LPVOID(offset), buffer, SIZE_T(size), bytesRead)
			if success == 0 && *bytesRead != SIZE_T(size) {
				log.Panic("读取内存失败!")
			}
		} else {
			sucess := ReadProcessMemory(pvz.ProcessHandle, LPVOID(offset), buffer, SIZE_T(readSize), bytesRead)
			if sucess == 0 && *bytesRead != SIZE_T(readSize) {
				log.Panic("读取内存失败!")
			}
		}
	}

	// log.Printf("读取内存, 地址 %v, 字节数 %d, 结果 %d.", address, readSize, *buffer)
	return *buffer
}

// @title: pvzWindow::WriteMemory
// @description: 写入内存
// @param: writeBuffer []byte 要写入的字节
// @param: writeSzie int 写入字节数
// @param: address ...int 内存地址(可以多级偏移)
// @return: void
func (pvz *pvzWindow) WriteMemory(writeBuffer []byte, writeSzie int, address ...int) {
	if !pvz.isValid() {
		log.Panic("窗口无效!")
	}

	// 加锁
	pvz.memoryLock <- struct{}{}
	defer func() {
		// 解锁
		<-pvz.memoryLock
	}()

	level := len(address)       // 偏移级数
	var offset LPVOID = 0       // 内存地址
	var buffer = new(LPVOID)    // 缓冲区
	var bytesRead = new(SIZE_T) // 读取字节数

	for i := 0; i < level; i++ {
		offset = *buffer + LPVOID(address[i])
		if i != level-1 {
			size := 4
			success := ReadProcessMemory(pvz.ProcessHandle, LPVOID(offset), buffer, SIZE_T(size), bytesRead)
			if success == 0 && *bytesRead != SIZE_T(size) {
				log.Panic("读取内存失败!")
			}
		} else {
			bytesWrite := new(SIZE_T)
			sucess := WriteProcessMemory(pvz.ProcessHandle, LPVOID(offset), LPVOID(unsafe.Pointer(&writeBuffer[0])), SIZE_T(writeSzie), bytesWrite)
			if sucess == 0 && *bytesWrite != SIZE_T(writeSzie) {
				log.Panic("写入内存失败!")
			}
		}
	}

	// log.Printf("写入内存, 地址 %v, 字节数 %d, 结果 %v.", address, writeSzie, writeBuffer)
}

// @title: ToBytes
// @description: 将任意类型转换为字节切片
// @param: val T 任意类型
// @return: []byte
func ToBytes[T interface{}](val T) []byte {
	size := unsafe.Sizeof(val)
	bytes := make([]byte, size)
	for i := 0; i < int(size); i++ {
		bytes[i] = *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(&val)) + uintptr(i)))
	}
	return bytes
}

// @title: pvzWindow::AutoCollect
// @description: 自动收集
// @param: on bool 开关
// @return: void
func (pvz *pvzWindow) AutoCollect(on bool) {
	if !pvz.isValid() {
		log.Panic("窗口无效!")
	}
	if on {
		pvz.WriteMemory(ToBytes(AUTO_COLLECT_ON), 1, AUTO_COLLECT_ADDR)
	} else {
		pvz.WriteMemory(ToBytes(AUTO_COLLECT_OFF), 1, AUTO_COLLECT_ADDR)
	}
}

// @title: pvzWindow::GetGameUI
// @description: 获取游戏界面类型
// @return: int 1: 主界面, 2: 选卡, 3: 正常游戏/战斗, 4: 僵尸进屋, 7: 模式选择, -1: 不可用
func (pvz *pvzWindow) GetGameUI() int {
	if !pvz.isValid() {
		return -1
	}
	return int(pvz.ReadMemory(4, 0x731C50, 0x7FC+0x120).(LPVOID))
}

func (pvz *pvzWindow) selectCard(card int) {
	if !pvz.isValid() {
		log.Panic("窗口无效!")
	}
	// 判断是否是模仿者
	isImitator := false
	imitatorType := 0
	if card >= 48 {
		isImitator = true
		imitatorType = card - 48
		card = 48
	}

	// 读取已选几张卡
	indexSelect := int(pvz.ReadMemory(4, 0x731C50, 0x774+0x100, 0xd24+0x18).(LPVOID))
	if indexSelect >= 10 {
		return
	}

	cd := &Code{
		page:      256,
		code:      make([]byte, 1024),
		length:    0,
		calls_pos: make([]uint16, 0),
	}
	if isImitator {
		// 写入要模仿的卡片类型
		pvz.WriteMemory(ToBytes(imitatorType), 4, 0x731C50, 0x774+0x100, 0xd8+0x18+0x3c*card)
		// call 0x00494690
		asm_mov_exx_dword_ptr(cd, EBP, 0x731C50)
		asm_mov_exx_exx(cd, EAX, EBP)
		asm_mov_exx_dword_ptr_exx_add(cd, EAX, 0x874)
		asm_mov_exx_dword_ptr_exx_add(cd, EBP, 0x874)
		// 写入add ebp,0xbfc
		asm_add_byte(cd, 129)
		asm_add_byte(cd, 197)
		asm_add_byte(cd, 252)
		asm_add_byte(cd, 11)
		asm_add_byte(cd, 0)
		asm_add_byte(cd, 0)
		// 写入完成
		asm_push_exx(cd, EBP)
		asm_call(cd, 0x00494690)
		asm_ret(cd)
		asm_code_inject(cd, pvz.ProcessHandle)
		log.Printf("选择卡片, 卡片类型 %d, 模仿类型 %d.", card, imitatorType)
	} else {
		asm_mov_exx_dword_ptr(cd, EBP, 0x731C50)
		asm_mov_exx_dword_ptr_exx_add(cd, EBP, 0x874)
		// 写入add ebp, val
		asm_add_byte(cd, 129)
		asm_add_byte(cd, 197)
		// 将val转换为字节数组
		var val uint32 = 0xa4 + 0x18 + 0x3c*uint32(card)
		temp := ToBytes(val)
		// 添加到code
		for _, v := range temp {
			asm_add_byte(cd, v)
		}
		// 写入完毕
		asm_mov_exx_exx(cd, EAX, EBP)
		asm_push_exx(cd, EAX)
		asm_mov_exx_dword_ptr(cd, EAX, 0x731C50)
		asm_mov_exx_dword_ptr_exx_add(cd, EAX, 0x874)
		asm_call(cd, 0x00494690)
		asm_ret(cd)
		asm_code_inject(cd, pvz.ProcessHandle)
		log.Printf("选择卡片, 卡片类型 %d.", card)
	}

	// // 获取移动坐标
	// x := pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0x144+0x18, 0x28+0x8+indexSelect*0x50).(LPVOID)
	// y := pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0x144+0x18, 0x28+0xc+indexSelect*0x50).(LPVOID)
	// // 写入移动坐标
	// pvz.WriteMemory(ToBytes(uint(x)), 4, 0x731C50, 0x774+0x100, 0xbc+0x18+0x3c*card)
	// pvz.WriteMemory(ToBytes(uint(y)), 4, 0x731C50, 0x774+0x100, 0xc0+0x18+0x3c*card)

	// // 如果是模仿者还需写入卡片类型
	// if isImitator {
	// 	pvz.WriteMemory(ToBytes(imitatorType), 4, 0x731C50, 0x774+0x100, 0xd8+0x18+0x3c*card)
	// }
	// // 写入卡片状态
	// pvz.WriteMemory(ToBytes(uint(0)), 4, 0x731C50, 0x774+0x100, 0xc8+0x18+0x3c*card)
	// // 写入已选几张卡
	// pvz.WriteMemory(ToBytes(uint(indexSelect+1)), 4, 0x731C50, 0x774+0x100, 0xd24+0x18)

}

func (pvz *pvzWindow) selectCards(card []int) {
	// 判断窗口是否有效
	if !pvz.isValid() {
		log.Panic("窗口无效!")
	}
	// 判断是否在选卡界面
	if pvz.GetGameUI() != 2 {
		log.Panic("不在选卡界面!")
	}

	// 获取卡槽数量
	cardNum := int(pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0x144+0x18, 0x24).(LPVOID))
	// 判断卡槽数量是否足够
	if len(card) > cardNum {
		log.Panic("卡槽数量不足!")
	}
	for _, v := range card {
		pvz.selectCard(v)
		time.Sleep(1 * 100 * time.Millisecond)
	}
}

// @title: pvzWindow::GetSlotsInfo
// @description: 获取卡槽信息
// @return: []int
func (pvz *pvzWindow) GetSlotsInfo() []int {
	if !pvz.isValid() {
		log.Panic("窗口无效!")
	}
	// 获取卡槽数量
	cardNum := int(pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0x144+0x18, 0x24).(LPVOID))
	var slotsInfo []int = make([]int, cardNum)
	// 先获取第一个卡槽的横坐标
	x0 := int(pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0x144+0x18, 0x28+0x8+0*0x50).(LPVOID))
	// 获取第二个卡槽的横坐标
	x1 := int(pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0x144+0x18, 0x28+0x8+1*0x50).(LPVOID))

	// 遍历所有卡片的信息
	for i := 0; i < 48; i++ {

		// 获取卡片状态,0移上卡槽,1在卡槽里,2移下卡槽,3在选卡界面里
		status := int(pvz.ReadMemory(4, 0x731C50, 0x774+0x100, 0xc8+0x18+0x3c*i).(LPVOID))
		if status == 1 {
			// 获取卡片横坐标
			x := int(pvz.ReadMemory(4, 0x731C50, 0x774+0x100, 0xbc+0x18+0x3c*i).(LPVOID))
			// 通过横坐标判断卡片在第几个卡槽
			index := (x - x0) / (x1 - x0)
			slotsInfo[index] = i
		}
	}

	// 获取模仿者状态
	status := int(pvz.ReadMemory(4, 0x731C50, 0x774+0x100, 0xc8+0x18+0x3c*48).(LPVOID))
	if status == 1 {
		// 获取卡片横坐标
		x := int(pvz.ReadMemory(4, 0x731C50, 0x774+0x100, 0xbc+0x18+0x3c*48).(LPVOID))
		// 通过横坐标判断卡片在第几个卡槽
		index := (x - x0) / (x1 - x0)
		// 判断模仿者类型
		imitatorType := int(pvz.ReadMemory(4, 0x731C50, 0x774+0x100, 0xd8+0x18+0x3c*48).(LPVOID))
		slotsInfo[index] = imitatorType + 48
	}
	return slotsInfo
}

// 自动冰消珊瑚(弃坑,写的太烂了)
func (pvz *pvzWindow) AutoIceShroom(on bool) {
	pvz.autoIceShroom = on
	// 开启一个协程
	go func() {
		// flag := false
		// for pvz.autoIceShroom {
		// 	// 读取当前游戏界面,判断是否在战斗
		// 	ui := pvz.GetGameUI()
		// 	if ui == 3 {
		// 		// 读取总波数
		// 		totalWave := int(pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0x5564+0x18).(LPVOID))
		// 		// 读取当前波数
		// 		currentWave := int(pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0x557C+0x18).(LPVOID))

		// 		// 如果当前波数位于最后一波之前
		// 		if currentWave == totalWave-1 {
		// 			// 获取下一波刷新倒计时
		// 			nextWaveTime := int(pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0x559C+0x18).(LPVOID))
		// 			// 如果下一波刷新倒计时小于等于150
		// 			if nextWaveTime <= 150 && !flag {
		// 				// 开启循环
		// 				for {
		// 					// 读取大波僵尸刷新倒计时
		// 					bigWaveTime := int(pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0x55A4+0x18).(LPVOID))
		// 					// 如果大波僵尸刷新倒计时小于等于300且不为0
		// 					if bigWaveTime <= 300 && bigWaveTime != 0 && !flag {
		// 						// 读取场上植物信息
		// 						plants := pvz.GetPlants()
		// 						// 获取一个空位
		// 						var row, col int
		// 						for i := 0; i < 6; i++ {
		// 							// 如果为水路则跳过
		// 							if i == 2 || i == 3 {
		// 								continue
		// 							}
		// 							for j := 0; j < 9; j++ {
		// 								if plants[i][j] == -1 {
		// 									row = i
		// 									col = j
		// 									break
		// 								}
		// 							}
		// 						}
		// 						// 放置冰消珊瑚
		// 						fmt.Println(flag)
		// 						pvz.PutPlant(row, col, 14)
		// 						pvz.PutPlant(row, col, 35)
		// 						flag = true
		// 						break
		// 					}
		// 					// 休眠0.5秒
		// 					time.Sleep(500 * time.Millisecond)
		// 				}
		// 			}
		// 		} else if currentWave == 0 {
		// 			// 否则重置flag
		// 			fmt.Println("重置")
		// 			flag = false
		// 		}
		// 	}

		// 	// 休眠1秒
		// 	time.Sleep(1 * time.Second)
		// }
	}()
}

// @title: pvzWindow::PutPlant
// @description: 放置植物
// @param: row int 行
// @param: col int 列
// @param: plantType int 植物类型
// @return: void
func (pvz *pvzWindow) PutPlant(row, col, plantType int) {
	if col < 0 || col > 9 || row < 0 || row > 5 || plantType < 0 {
		return
	}
	// 判断是否战斗界面
	if pvz.GetGameUI() != 3 {
		return
	}

	cd := &Code{
		page:      256,
		code:      make([]byte, 1024),
		length:    0,
		calls_pos: make([]uint16, 0),
	}
	// 判断是否是模仿者
	if plantType > 48 {
		plantType = plantType % 48
		asm_push[uint32](cd, uint32(plantType))
		asm_push[uint32](cd, 48)
	} else {
		asm_push[uint32](cd, 0xffffffff)
		asm_push[uint32](cd, uint32(plantType))
	}

	asm_mov_exx[uint32](cd, EAX, uint32(row))
	asm_push[uint32](cd, uint32(col))
	asm_mov_exx_dword_ptr(cd, EBP, 0x731C50)
	asm_mov_exx_dword_ptr_exx_add(cd, EBP, 0x768+0x100)
	asm_push_exx(cd, EBP)
	asm_call(cd, 0x004105a0)
	asm_ret(cd)
	asm_code_inject(cd, pvz.ProcessHandle)
}

func (pvz *pvzWindow) GetPlants() [6][9]int {
	Plants := [6][9]int{}
	// 初始化为-1
	for i := 0; i < 6; i++ {
		for j := 0; j < 9; j++ {
			Plants[i][j] = -1
		}
	}
	// 读取场上有几株植物
	plantNum := int(pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0xbc+0x18).(LPVOID))
	// 遍历所有植物
	for i := 0; i < plantNum; i++ {
		// 获取所在行数
		row := int(pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0xac+0x18, 0x1c+0x14c*i).(LPVOID))
		// 获取所在列数
		col := int(pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0xac+0x18, 0x28+0x14c*i).(LPVOID))
		// 获取植物类型
		plantType := int(pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0xac+0x18, 0x24+0x14c*i).(LPVOID))
		// 将植物类型写入数组
		Plants[row][col] = plantType
	}
	return Plants

}
