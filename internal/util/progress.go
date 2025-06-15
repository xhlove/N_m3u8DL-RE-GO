package util

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ProgressTask 进度任务
type ProgressTask struct {
	ID          int
	Description string
	Value       float64
	MaxValue    float64
	IsStarted   bool
	IsFinished  bool
	startTime   time.Time
	finishTime  time.Time
	mutex       sync.RWMutex
}

// NewProgressTask 创建新的进度任务
func NewProgressTask(id int, description string) *ProgressTask {
	return &ProgressTask{
		ID:          id,
		Description: description,
		MaxValue:    1.0, // 默认为1，后续会被设置为实际分段数
		Value:       0.0,
		IsStarted:   false,
		IsFinished:  false,
	}
}

// StartTask 开始任务
func (pt *ProgressTask) StartTask() {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	if !pt.IsStarted {
		pt.IsStarted = true
		pt.startTime = time.Now()
	}
}

// Increment 增加进度
func (pt *ProgressTask) Increment(amount float64) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.Value += amount
	if pt.Value >= pt.MaxValue && !pt.IsFinished {
		pt.IsFinished = true
		pt.finishTime = time.Now()
	}
}

// SetValue 设置当前值
func (pt *ProgressTask) SetValue(value float64) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.Value = value
	// 使用更宽松的完成条件，考虑浮点数精度问题
	if (pt.Value >= pt.MaxValue || pt.Value >= pt.MaxValue-0.1) && !pt.IsFinished {
		pt.IsFinished = true
		pt.Value = pt.MaxValue // 确保显示为完整值
		pt.finishTime = time.Now()
	}
}

// SetMaxValue 设置最大值
func (pt *ProgressTask) SetMaxValue(maxValue float64) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.MaxValue = maxValue
}

// GetPercentage 获取百分比
func (pt *ProgressTask) GetPercentage() float64 {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()

	if pt.MaxValue <= 0 {
		return 0.0
	}
	percentage := (pt.Value / pt.MaxValue) * 100.0
	if percentage > 100.0 {
		percentage = 100.0
	}
	return percentage
}

// GetElapsedTime 获取已用时间
func (pt *ProgressTask) GetElapsedTime() time.Duration {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()

	if !pt.IsStarted {
		return 0
	}

	endTime := time.Now()
	if pt.IsFinished {
		endTime = pt.finishTime
	}

	return endTime.Sub(pt.startTime)
}

// SpeedContainer 速度容器，类似C#版本
type SpeedContainer struct {
	Downloaded     int64
	ResponseLength *int64
	RDownloaded    int64
	NowSpeed       int64
	LowSpeedCount  int
	SingleSegment  bool
	ShouldStop     bool
	SpeedLimit     int64
	speedMutex     sync.RWMutex
	lastResetTime  time.Time
	recorder       []int64 // 记录最近几秒的下载量
	maxRecordCount int
}

// NewSpeedContainer 创建速度容器
func NewSpeedContainer() *SpeedContainer {
	return &SpeedContainer{
		speedMutex:     sync.RWMutex{},
		lastResetTime:  time.Now(),
		recorder:       make([]int64, 0, 10),
		maxRecordCount: 10,
	}
}

// Add 增加下载量
func (sc *SpeedContainer) Add(bytes int64) {
	sc.speedMutex.Lock()
	defer sc.speedMutex.Unlock()

	sc.Downloaded += bytes
	sc.RDownloaded += bytes
}

// Reset 重置速度计算
func (sc *SpeedContainer) Reset() {
	sc.speedMutex.Lock()
	defer sc.speedMutex.Unlock()

	// 记录这一秒的下载量
	if len(sc.recorder) >= sc.maxRecordCount {
		sc.recorder = sc.recorder[1:]
	}
	sc.recorder = append(sc.recorder, sc.Downloaded)

	sc.Downloaded = 0
	sc.lastResetTime = time.Now()
}

// ResetVars 重置变量
func (sc *SpeedContainer) ResetVars() {
	sc.speedMutex.Lock()
	defer sc.speedMutex.Unlock()

	sc.Downloaded = 0
	sc.RDownloaded = 0
	sc.NowSpeed = 0
	sc.LowSpeedCount = 0
	sc.lastResetTime = time.Now()
	sc.recorder = sc.recorder[:0]
}

// AddLowSpeedCount 增加低速计数
func (sc *SpeedContainer) AddLowSpeedCount() {
	sc.speedMutex.Lock()
	defer sc.speedMutex.Unlock()

	sc.LowSpeedCount++
}

// ResetLowSpeedCount 重置低速计数
func (sc *SpeedContainer) ResetLowSpeedCount() {
	sc.speedMutex.Lock()
	defer sc.speedMutex.Unlock()

	sc.LowSpeedCount = 0
}

// GetSpeed 获取当前速度
func (sc *SpeedContainer) GetSpeed() int64 {
	sc.speedMutex.RLock()
	defer sc.speedMutex.RUnlock()

	// 基于最近的下载记录计算平均速度
	if len(sc.recorder) == 0 {
		return sc.NowSpeed
	}

	var total int64
	for _, bytes := range sc.recorder {
		total += bytes
	}

	if len(sc.recorder) > 0 {
		avgSpeed := total / int64(len(sc.recorder))
		if avgSpeed > sc.NowSpeed {
			sc.NowSpeed = avgSpeed
		}
	}

	return sc.NowSpeed
}

// UpdateSpeed 更新速度计算
func (sc *SpeedContainer) UpdateSpeed() {
	sc.speedMutex.Lock()
	defer sc.speedMutex.Unlock()

	// 计算自上次重置以来的时间间隔
	now := time.Now()
	elapsed := now.Sub(sc.lastResetTime)

	// 如果时间间隔大于等于1秒，才进行速度计算
	if elapsed >= time.Second {
		// 计算这段时间的平均速度 (字节/秒)
		if elapsed.Seconds() > 0 {
			currentSpeed := int64(float64(sc.Downloaded) / elapsed.Seconds())

			// 记录当前速度
			if len(sc.recorder) >= sc.maxRecordCount {
				sc.recorder = sc.recorder[1:]
			}
			sc.recorder = append(sc.recorder, currentSpeed)

			// 计算平均速度
			if len(sc.recorder) > 0 {
				var total int64
				for _, speed := range sc.recorder {
					total += speed
				}
				sc.NowSpeed = total / int64(len(sc.recorder))
			}
		}

		// 重置计数和时间
		sc.Downloaded = 0
		sc.lastResetTime = now
	}
}

// Progress 进度管理器
type Progress struct {
	tasks           map[int]*ProgressTask
	speedContainers map[int]*SpeedContainer
	taskOrder       []int // 保持任务显示顺序
	taskCounter     int
	isStarted       bool
	autoRefresh     bool
	mutex           sync.RWMutex
	ticker          *time.Ticker
	stopCh          chan bool
	renderer        *FixedPositionRenderer
	allFinished     bool
	finishedTime    time.Time
	finalDisplayed  bool // 标记是否已经显示了最终状态
}

// FixedPositionRenderer 固定位置渲染器
type FixedPositionRenderer struct {
	spinnerChars []string
	taskSpinners map[int]int // 每个任务的旋转状态
	spinnerMutex sync.RWMutex
	cursorSaved  bool // 是否已保存光标位置
}

func NewFixedPositionRenderer() *FixedPositionRenderer {
	return &FixedPositionRenderer{
		spinnerChars: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		taskSpinners: make(map[int]int),
		spinnerMutex: sync.RWMutex{},
		cursorSaved:  false,
	}
}

// RenderProgressLine 渲染单行进度
func (r *FixedPositionRenderer) RenderProgressLine(task *ProgressTask, speedContainer *SpeedContainer) string {
	task.mutex.RLock()
	defer task.mutex.RUnlock()

	// 1. 流信息部分 - 从task description解析 (限制长度)
	streamInfo := task.Description
	if len(streamInfo) > 30 {
		streamInfo = streamInfo[:27] + "..."
	}

	// 2. 进度条部分 (30个字符宽度，带颜色)
	percentage := task.GetPercentage() / 100.0
	barWidth := 30
	filledWidth := int(percentage * float64(barWidth))

	// 确保已完成任务显示完整进度条
	if task.IsFinished {
		filledWidth = barWidth
		percentage = 1.0
	}

	var progressBar strings.Builder
	for i := 0; i < barWidth; i++ {
		if i < filledWidth {
			// 充满部分 - 已完成用绿色，进行中用蓝色
			if task.IsFinished {
				progressBar.WriteString(Console.Colorize("━", Green))
			} else {
				progressBar.WriteString(Console.Colorize("━", Blue))
			}
		} else {
			// 空白部分 - 暗灰色
			progressBar.WriteString(Console.Colorize("━", BrightBlack))
		}
	}

	// 3. 分段进度 - 带颜色
	var segmentProgress string
	if task.IsFinished {
		segmentProgress = Console.Colorize(fmt.Sprintf("%3.0f/%3.0f 100.00%%",
			task.MaxValue, task.MaxValue), Green)
	} else {
		segmentProgress = fmt.Sprintf("%3.0f/%3.0f %5.2f%%",
			task.Value, task.MaxValue, task.GetPercentage())
	}

	// 4. 文件大小进度
	var sizeProgress string
	speedContainer.speedMutex.RLock()
	downloaded := speedContainer.RDownloaded
	total := speedContainer.ResponseLength
	speedContainer.speedMutex.RUnlock()

	if total != nil && *total > 0 {
		sizeProgress = fmt.Sprintf("%s", FormatFileSize(downloaded))
		if !task.IsFinished {
			sizeProgress = fmt.Sprintf("%s/%s", FormatFileSize(downloaded), FormatFileSize(*total))
		}
	} else if downloaded > 0 {
		sizeProgress = FormatFileSize(downloaded)
	} else {
		sizeProgress = "-"
	}
	// 固定长度
	if len(sizeProgress) > 18 {
		sizeProgress = sizeProgress[:15] + "..."
	}

	// 5. 速度 - 带颜色
	var speedStr string
	if task.IsFinished {
		// 计算整个下载的平均速度
		elapsed := task.GetElapsedTime()
		if elapsed.Seconds() > 0 && downloaded > 0 {
			avgSpeed := int64(float64(downloaded) / elapsed.Seconds())
			speedStr = Console.Colorize(FormatFileSize(avgSpeed)+"ps", Green)
		} else {
			speedStr = Console.Colorize("-", Green)
		}
	} else if !task.IsStarted {
		speedStr = Console.Colorize("-", BrightBlack)
	} else {
		speed := speedContainer.GetSpeed()
		if speed > 0 {
			speedStr = Console.Colorize(FormatFileSize(speed)+"ps", Cyan)
		} else {
			speedStr = Console.Colorize("-", BrightBlack)
		}
	}

	// 6. 剩余时间 - 带颜色
	var timeStr string
	if task.IsFinished {
		timeStr = Console.Colorize(formatDurationCSharp(task.GetElapsedTime()), Green)
	} else if !task.IsStarted {
		timeStr = Console.Colorize("--:--:--", BrightBlack)
	} else {
		elapsed := task.GetElapsedTime()
		completed := task.Value / task.MaxValue

		// 根据已完成进度和已用时间计算剩余时间
		if completed >= 0.005 && elapsed.Seconds() >= 2.0 { // 至少0.5%进度且至少2秒
			// 计算每个单元的平均时间
			avgTimePerUnit := elapsed.Seconds() / task.Value
			remainingUnits := task.MaxValue - task.Value
			remainingSeconds := avgTimePerUnit * remainingUnits

			if remainingSeconds > 0 && remainingSeconds < 24*3600 { // 限制在24小时内
				remaining := time.Duration(remainingSeconds) * time.Second
				timeStr = Console.Colorize(formatDurationCSharp(remaining), Yellow)
			} else {
				timeStr = Console.Colorize("--:--:--", BrightBlack)
			}
		} else {
			timeStr = Console.Colorize("--:--:--", BrightBlack)
		}
	}

	// 7. 旋转指示器 - 带颜色
	var spinner string
	if task.IsFinished {
		spinner = Console.Colorize("✓", Green)
	} else if task.IsStarted {
		// 为每个任务维护独立的旋转状态
		r.spinnerMutex.Lock()
		spinnerIndex, exists := r.taskSpinners[task.ID]
		if !exists {
			spinnerIndex = 0
		}
		spinnerChar := r.spinnerChars[spinnerIndex%len(r.spinnerChars)]
		r.taskSpinners[task.ID] = spinnerIndex + 1
		r.spinnerMutex.Unlock()

		spinner = Console.Colorize(spinnerChar, Blue)
	} else {
		spinner = Console.Colorize("⠀", BrightBlack)
	}

	// 组合所有部分 - 类似C#版本的格式
	return fmt.Sprintf("%-30s %s %s %18s %9s %8s %s",
		streamInfo,
		progressBar.String(),
		segmentProgress,
		sizeProgress,
		speedStr,
		timeStr,
		spinner)
}

// Render 渲染所有进度行
func (r *FixedPositionRenderer) Render(tasks map[int]*ProgressTask, speedContainers map[int]*SpeedContainer, taskOrder []int) {
	// 第一次渲染时，保存光标位置并预留空间
	if !r.cursorSaved {
		fmt.Print("\033[s") // 保存光标位置
		// 为每个任务预留一行
		for range taskOrder {
			fmt.Println()
		}
		r.cursorSaved = true
	}

	// 恢复到保存的光标位置
	fmt.Print("\033[u") // 恢复光标位置

	// 逐行渲染每个任务
	for i, taskID := range taskOrder {
		task, exists := tasks[taskID]
		if !exists {
			// 如果任务不存在，输出空行
			fmt.Print("\033[2K\n") // 清除当前行并换行
			continue
		}

		speedContainer := speedContainers[taskID]
		if speedContainer == nil {
			speedContainer = NewSpeedContainer()
		}

		line := r.RenderProgressLine(task, speedContainer)
		fmt.Print("\033[2K") // 清除当前行
		fmt.Print(line)

		// 如果不是最后一行，换行
		if i < len(taskOrder)-1 {
			fmt.Print("\n")
		}
	}
}

// NewProgress 创建进度管理器
func NewProgress() *Progress {
	return &Progress{
		tasks:           make(map[int]*ProgressTask),
		speedContainers: make(map[int]*SpeedContainer),
		taskOrder:       make([]int, 0),
		taskCounter:     0,
		autoRefresh:     true,
		stopCh:          make(chan bool, 1),
		renderer:        NewFixedPositionRenderer(),
		allFinished:     false,
		finalDisplayed:  false,
	}
}

// AddTask 添加任务
func (p *Progress) AddTask(description string) *ProgressTask {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.taskCounter++
	task := NewProgressTask(p.taskCounter, description)
	p.tasks[p.taskCounter] = task
	p.speedContainers[p.taskCounter] = NewSpeedContainer()
	p.taskOrder = append(p.taskOrder, p.taskCounter) // 保持顺序
	return task
}

// GetSpeedContainer 获取任务的速度容器
func (p *Progress) GetSpeedContainer(taskID int) *SpeedContainer {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if container, exists := p.speedContainers[taskID]; exists {
		return container
	}
	return NewSpeedContainer()
}

// StartAsync 开始异步显示进度
func (p *Progress) StartAsync(callback func(*Progress)) {
	p.mutex.Lock()
	p.isStarted = true
	p.mutex.Unlock()

	// 启动刷新定时器
	if p.autoRefresh {
		p.ticker = time.NewTicker(200 * time.Millisecond)
		go p.refreshLoop()
	}

	// 执行回调
	callback(p)

	// 停止刷新
	p.Stop()
}

// refreshLoop 刷新循环
func (p *Progress) refreshLoop() {
	for {
		select {
		case <-p.ticker.C:
			// 更新所有任务的速度计算
			p.updateSpeeds()
			p.render()
		case <-p.stopCh:
			return
		}
	}
}

// updateSpeeds 更新所有任务的速度
func (p *Progress) updateSpeeds() {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	for _, speedContainer := range p.speedContainers {
		if speedContainer != nil {
			speedContainer.UpdateSpeed()
		}
	}
}

// render 渲染进度显示
func (p *Progress) render() {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if len(p.tasks) == 0 {
		return
	}

	// 检查是否所有任务都完成了
	allFinished := true
	for _, task := range p.tasks {
		if !task.IsFinished {
			allFinished = false
			break
		}
	}

	// 渲染进度 - 无论是否完成都要继续渲染
	p.renderer.Render(p.tasks, p.speedContainers, p.taskOrder)

	// 如果所有任务都完成了，记录完成时间，但继续显示进度条
	// 参考C#版本：进度条保持显示直到所有合并完成
	if allFinished && !p.allFinished {
		p.allFinished = true
		p.finishedTime = time.Now()
		// 不立即停止，让进度条继续显示，直到外部调用Stop()
	}
}

// Stop 停止进度显示
func (p *Progress) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.ticker != nil {
		p.ticker.Stop()
		// 非阻塞发送停止信号
		select {
		case p.stopCh <- true:
		default:
		}
	}
	p.isStarted = false

	// 最后渲染一次确保显示最终状态，但只显示一次
	if p.renderer != nil && !p.finalDisplayed {
		p.renderer.Render(p.tasks, p.speedContainers, p.taskOrder)
		p.finalDisplayed = true
		// 在最终输出后添加一个空行，避免后续日志覆盖进度条
		fmt.Println()
	}
}

// formatDurationCSharp 格式化时间间隔 (C#风格，总是显示小时)
func formatDurationCSharp(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
