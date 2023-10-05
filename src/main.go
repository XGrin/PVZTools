package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// 退出标志
var isExit = false

// pvz窗口
var pvz = &pvzWindow{
	Handle:        0,
	Pid:           0,
	ProcessHandle: 0,
	memoryLock:    make(chan struct{}, 1),
}

// 存储创建的句柄
var handleList []HANDLE

// 选卡列表
var selectList = []int{14, 15, 2, 16, 17, 30, 35, 37, 34, 47}

func main() {
	log.SetPrefix("[main] ")
	log.Println("程序启动!")

	pvz.AutoIceShroom(true)

	processInfo := binding.NewString()
	inputText := binding.NewString()
	inputText.Set("14, 15, 2, 16, 17, 30, 35, 37, 34, 47")
	inputText.AddListener(binding.NewDataListener(func() {
		// 分割输入的字符串
		inputStr, _ := inputText.Get()
		inputList := strings.Split(inputStr, ",")
		// 清空选卡列表
		selectList = selectList[:0]
		for _, v := range inputList {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			// 转换为int
			vInt, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				continue
			}
			selectList = append(selectList, int(vInt))
		}

	}))

	app := app.New()
	w := app.NewWindow("PVZTool")
	w.Resize(fyne.NewSize(400, 250))

	// 自动收集选择框
	autoCollectCheck := widget.NewCheck("Auto Collect", func(on bool) {
		if !pvz.isValid() {
			return
		}
		pvz.AutoCollect(on)
	})
	// 自动选卡选择框
	autoSelectChan := make(chan bool, 1)
	autoSelectCheck := widget.NewCheck("Auto Select", func(on bool) {
		if on {
			if !pvz.isValid() {
				return
			}
			// 开启一个协程进行自动选卡
			go func() {
				// 循环监听是否进入选卡界面, 如果进入则自动选择卡片
				for {
					// 读取选卡界面状态
					ui := pvz.GetGameUI()
					if ui == 2 {
						// 读取选卡界面横向偏移
						offsetX := int(pvz.ReadMemory(4, 0x731C50, 0x768+0x100, 0x15C+0x18, 0x8).(LPVOID))

						// 判断是否选取完毕
						indexSelect := int(pvz.ReadMemory(4, 0x731C50, 0x774+0x100, 0xd24+0x18).(LPVOID))
						if indexSelect == 0 && offsetX == 4250 {
							// 选取卡片
							pvz.selectCards(selectList)
						}
					}
					select {
					case <-autoSelectChan:
						return
					default:
					}
					time.Sleep(time.Second * 2)

				}
			}()
		} else {
			autoSelectChan <- on
		}

	})

	// 选卡输入框
	selectInput := widget.NewEntryWithData(inputText)

	// 获取卡槽信息按钮
	getSlotsInfoButton := widget.NewButton("Get Slots Info", func() {
		if !pvz.isValid() {
			return
		}
		if pvz.GetGameUI() != 2 {
			return
		}
		// 获取卡槽信息
		slotsInfo := pvz.GetSlotsInfo()
		// 将卡槽信息转换为字符串
		slotsInfoStr := ""
		for _, v := range slotsInfo {
			slotsInfoStr += fmt.Sprintf("%d, ", v)
		}
		// 显示卡槽信息
		inputText.Set(slotsInfoStr)
	})

	// // 自动冰消珊瑚选择框
	// autoIceShroomCheck := widget.NewCheck("Auto Ice Shroom", func(on bool) {
	// 	if !pvz.isValid() {
	// 		return
	// 	}
	// 	pvz.AutoIceShroom(on)
	// })

	// 循环查找进程
	go func() {
		for {
			if !pvz.isValid() {
				pvz.Handle = FindWindow("MainWindow", "Plants vs. Zombies")
				if pvz.Handle != 0 {
					GetWindowThreadProcessId(pvz.Handle, &pvz.Pid)
					pvz.ProcessHandle = OpenProcess(PROCESS_ALL_ACCESS, 0, pvz.Pid)
					processInfo.Set(fmt.Sprint("Find PVZ ", "Process ID: ", pvz.Pid, " Process Handle: ", pvz.ProcessHandle))
					handleList = append(handleList, pvz.ProcessHandle)
					// 更新autoCollectCheck的状态
					autoCollectCheck.Enable()
					// 更新autoSelectCheck的状态
					autoSelectCheck.Enable()
					// 更新autoIceShroomCheck的状态
					// autoIceShroomCheck.Enable()
				} else {
					processInfo.Set("Can not find PVZ process!")
					// 更新autoCollectCheck的状态
					autoCollectCheck.SetChecked(false)
					autoCollectCheck.Disable()
					// 更新autoSelectCheck的状态
					autoSelectCheck.SetChecked(false)
					autoSelectCheck.Disable()
					// 更新autoIceShroomCheck的状态
					// autoIceShroomCheck.SetChecked(false)
					// autoIceShroomCheck.Disable()
				}

			}
			time.Sleep(time.Second * 2)
			if isExit {
				return
			}
		}
	}()

	// 进程信息
	processInfoLabel := widget.NewLabelWithData(processInfo)

	content := container.New(layout.NewVBoxLayout(), autoCollectCheck, autoSelectCheck, selectInput, getSlotsInfoButton, processInfoLabel)
	w.SetContent(content)
	w.ShowAndRun()
	isExit = true
	// 关闭句柄
	defer func() {
		for _, v := range handleList {
			CloseHandle(v)
			log.Println("关闭句柄", v)
		}
	}()

	// 进程结束关闭自动收集
	defer pvz.AutoCollect(false)
}

// func main() {
// 	fmt.Println("start")
// 	pvz.Handle = FindWindow("MainWindow", "Plants vs. Zombies")
// 	GetWindowThreadProcessId(pvz.Handle, &pvz.Pid)
// 	pvz.ProcessHandle = OpenProcess(PROCESS_ALL_ACCESS, 0, pvz.Pid)
// 	defer CloseHandle(pvz.ProcessHandle)
// 	pvz.selectCard(2)

// }
