package util

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const (
	TaskTypeDownload = "Download"
	TaskTypeDecrypt  = "Decrypt"
	TaskTypeMerge    = "Merge"
	TaskTypeMux      = "Mux"
)

// --- Task ---

type Task struct {
	ID             int
	Type           string
	Description    string
	Value          float64 // For progress bar calculation (e.g., processed segments)
	Total          float64 // For progress bar calculation (e.g., total segments)
	ProcessedCount int64   // For x/y display
	TotalCount     int64   // For x/y display
	CurrentBytes   int64   // For real-time size display
	TotalBytes     int64   // For average speed calculation
	IsStarted      bool
	IsFinished     bool
	IsError        bool
	Error          error
	startTime      time.Time
	finishTime     time.Time
	mutex          sync.RWMutex
	speedContainer *SpeedContainer
}

func (t *Task) GetSpeedContainer() *SpeedContainer {
	return t.speedContainer
}

func (t *Task) Update(value float64, currentBytes ...int64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.IsFinished {
		return
	}
	t.Value = value
	if len(currentBytes) > 0 {
		t.CurrentBytes = currentBytes[0]
	}

	if t.Value >= t.Total {
		t.Value = t.Total
		t.IsFinished = true
		t.finishTime = time.Now()
	}
}

func (t *Task) Increment(amount float64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.IsFinished {
		return
	}
	t.Value += amount
	t.ProcessedCount++
	if t.Value >= t.Total {
		t.Value = t.Total
		t.ProcessedCount = t.TotalCount
		t.IsFinished = true
		t.finishTime = time.Now()
	}
}

func (t *Task) SetError(err error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.IsError = true
	t.Error = err
	t.IsFinished = true
	t.finishTime = time.Now()
}

func (t *Task) render() string {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	// Styles
	grey := lipgloss.Color("240")
	green := lipgloss.Color("46") // Vibrant Green
	lightGreen := lipgloss.Color("119")
	white := lipgloss.Color("255")
	purple := lipgloss.Color("99")
	red := lipgloss.Color("196")
	yellow := lipgloss.Color("226") // Bright Yellow

	// Determine color based on state
	barColor := grey               // Default for not-yet-started tasks
	bgColor := lipgloss.Color("0") // Black background for finished tasks
	useBg := false

	if t.IsError {
		barColor = red
	} else if t.IsFinished {
		useBg = true
		switch t.Type {
		case TaskTypeDownload:
			barColor = green
		case TaskTypeDecrypt:
			barColor = lightGreen
		case TaskTypeMerge:
			barColor = white
		case TaskTypeMux:
			barColor = purple
		}
	} else if t.IsStarted {
		barColor = yellow // In-progress tasks are yellow
	}
	// Removed the blue color logic for in-progress tasks to stick to "light grey" requirement.

	// Progress Bar
	barWidth := 30
	percent := 0.0
	if t.Total > 0 {
		percent = t.Value / t.Total
	}
	if percent > 1.0 {
		percent = 1.0
	}
	filledWidth := int(float64(barWidth) * percent)

	var bar strings.Builder
	for i := 0; i < barWidth; i++ {
		style := lipgloss.NewStyle()
		if i < filledWidth {
			style = style.Foreground(barColor)
			if useBg {
				style = style.Background(bgColor)
			}
			bar.WriteString(style.Render("━"))
		} else {
			// For the unfilled part, always use grey foreground
			bar.WriteString(lipgloss.NewStyle().Foreground(grey).Render("━"))
		}
	}

	// Other info
	desc := t.Description
	if len(desc) > 32 {
		desc = desc[:30] + ".."
	}

	percentStr := fmt.Sprintf("%6.2f%%", percent*100)
	if t.IsFinished && !t.IsError {
		if t.Type == TaskTypeMerge && t.CurrentBytes > 0 && t.IsFinished && !t.IsError {
			// Special case for finished merge tasks: show ratio of pre-merge to post-merge size
			mergePercent := (float64(t.TotalBytes) / float64(t.CurrentBytes)) * 100
			percentStr = fmt.Sprintf("%6.2f%%", mergePercent)
		} else if t.IsFinished && !t.IsError { // Keep original 100% for other finished tasks
			percentStr = "100.00%"
		}
		// Note: If a merge task finishes with t.CurrentBytes == 0 (error before merge or empty output),
		// it will not enter the new 'if' and will fall through, potentially showing non-100% if percent was not 1.
		// This might need further refinement if that scenario needs specific handling.
		// For now, an error merge task would show its error color and its last 'percent' before error.
	}

	// Count, Percent, Size
	countStr := fmt.Sprintf("%d/%d", t.ProcessedCount, t.TotalCount)
	sizeStr := FormatSize(t.CurrentBytes)

	// Speed and Time info
	var speedStr, timeStr string
	if t.IsStarted && !t.IsFinished {
		// In-progress task
		duration := time.Now().Sub(t.startTime)
		timeStr = formatDuration(duration)
		t.speedContainer.UpdateSpeed()
		speed := t.speedContainer.GetSpeed()
		speedStr = FormatSize(speed) + "/s"
		t.CurrentBytes = t.speedContainer.RDownloaded // Update current bytes for download tasks
	} else if t.IsFinished {
		// Finished task
		duration := t.finishTime.Sub(t.startTime)
		timeStr = formatDuration(duration)
		if duration.Seconds() > 0 && t.TotalBytes > 0 {
			avgSpeed := float64(t.TotalBytes) / duration.Seconds()
			speedStr = FormatSize(int64(avgSpeed)) + "/s"
		}
		if t.CurrentBytes < t.TotalBytes { // Ensure final size is displayed correctly
			t.CurrentBytes = t.TotalBytes
		}
		sizeStr = FormatSize(t.CurrentBytes)
	}

	var typeAbbr string
	switch t.Type {
	case TaskTypeDownload:
		typeAbbr = "DL"
	case TaskTypeDecrypt:
		typeAbbr = "DEC"
	case TaskTypeMerge:
		typeAbbr = "MRG"
	case TaskTypeMux:
		typeAbbr = "MUX"
	default:
		typeAbbr = t.Type
	}

	return fmt.Sprintf("%-3s %-32s %s %8s %7s %9s %11s %12s",
		typeAbbr, desc, bar.String(), countStr, percentStr, sizeStr, speedStr, timeStr)
}

// FormatSize converts a size in bytes to a human-readable string.
func FormatSize(bytes int64) string {
	const (
		_          = iota
		KB float64 = 1 << (10 * iota)
		MB
		GB
		TB
	)

	switch {
	case float64(bytes) >= TB:
		return fmt.Sprintf("%.2fTB", float64(bytes)/TB)
	case float64(bytes) >= GB:
		return fmt.Sprintf("%.2fGB", float64(bytes)/GB)
	case float64(bytes) >= MB:
		return fmt.Sprintf("%.2fMB", float64(bytes)/MB)
	case float64(bytes) >= KB:
		return fmt.Sprintf("%.2fKB", float64(bytes)/KB)
	}
	return fmt.Sprintf("%dB", bytes)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Millisecond)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	d -= s * time.Second
	ms := d / time.Millisecond
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

// --- UIManager ---

type UIManager struct {
	tasks         map[int]*Task
	taskOrder     []int
	taskCounter   int
	taskMutex     sync.RWMutex
	renderMutex   sync.Mutex
	stopCh        chan struct{}
	stopOnce      sync.Once
	lastRenderLen int
}

var UI *UIManager

func init() {
	UI = &UIManager{
		tasks:     make(map[int]*Task),
		taskOrder: make([]int, 0),
		stopCh:    make(chan struct{}),
	}
}

func (ui *UIManager) AddTask(taskType, description string, totalCount int64, totalBytes int64) *Task {
	ui.taskMutex.Lock()
	defer ui.taskMutex.Unlock()
	ui.taskCounter++
	task := &Task{
		ID:             ui.taskCounter,
		Type:           taskType,
		Description:    description,
		Total:          float64(totalCount),
		TotalCount:     totalCount,
		TotalBytes:     totalBytes,
		startTime:      time.Now(),
		IsStarted:      true,
		speedContainer: NewSpeedContainer(),
	}
	ui.tasks[task.ID] = task
	ui.taskOrder = append(ui.taskOrder, task.ID)
	return task
}

func (ui *UIManager) Log(message string) {
	ui.renderMutex.Lock()
	defer ui.renderMutex.Unlock()

	ui.clearProgressArea()
	Console.MarkupLine(message)
	ui.drawProgress()
}

func (ui *UIManager) Start() {
	go ui.renderLoop()
}

func (ui *UIManager) renderLoop() {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ui.render()
		case <-ui.stopCh:
			ui.render() // Final render
			return
		}
	}
}

// render acquires the lock and calls the drawing functions.
func (ui *UIManager) render() {
	ui.renderMutex.Lock()
	defer ui.renderMutex.Unlock()

	ui.clearProgressArea()
	ui.drawProgress()
}

// clearProgressArea clears the lines previously rendered for the progress bars.
// It should only be called by a function that already holds the renderMutex.
func (ui *UIManager) clearProgressArea() {
	if ui.lastRenderLen > 0 {
		fmt.Printf("\033[%dA", ui.lastRenderLen)
		fmt.Print("\033[J")
	}
}

// drawProgress renders the current state of all tasks.
// It should only be called by a function that already holds the renderMutex.
func (ui *UIManager) drawProgress() {
	ui.taskMutex.RLock()
	defer ui.taskMutex.RUnlock()

	var renderedTasks []string
	for _, id := range ui.taskOrder {
		if task, ok := ui.tasks[id]; ok {
			renderedTasks = append(renderedTasks, task.render())
		}
	}

	if len(renderedTasks) > 0 {
		fmt.Println(strings.Join(renderedTasks, "\n"))
	}

	ui.lastRenderLen = len(renderedTasks)
}

func (ui *UIManager) Stop() {
	ui.stopOnce.Do(func() {
		close(ui.stopCh)
		// Give it a moment for the final render to complete
		time.Sleep(200 * time.Millisecond)

		ui.renderMutex.Lock()
		defer ui.renderMutex.Unlock()

		// Do a final render to show the completed state
		ui.clearProgressArea()
		ui.drawProgress()

		// After this, logging will resume as normal below the progress bars.
		// The next log will print on a new line, so we don't need an extra Println here.
	})
}

// --- SpeedContainer ---

type SpeedContainer struct {
	Downloaded     int64
	ResponseLength *int64
	RDownloaded    int64
	NowSpeed       int64
	speedMutex     sync.RWMutex
	lastResetTime  time.Time
	recorder       []int64
	maxRecordCount int
}

func NewSpeedContainer() *SpeedContainer {
	return &SpeedContainer{
		lastResetTime:  time.Now(),
		recorder:       make([]int64, 0, 10),
		maxRecordCount: 10,
	}
}

func (sc *SpeedContainer) Add(bytes int64) {
	sc.speedMutex.Lock()
	defer sc.speedMutex.Unlock()
	sc.Downloaded += bytes
	sc.RDownloaded += bytes
}

func (sc *SpeedContainer) GetSpeed() int64 {
	sc.speedMutex.RLock()
	defer sc.speedMutex.RUnlock()
	return sc.NowSpeed
}

func (sc *SpeedContainer) UpdateSpeed() {
	sc.speedMutex.Lock()
	defer sc.speedMutex.Unlock()
	elapsed := time.Since(sc.lastResetTime)
	if elapsed >= time.Second {
		currentSpeed := int64(float64(sc.Downloaded) / elapsed.Seconds())
		if len(sc.recorder) >= sc.maxRecordCount {
			sc.recorder = sc.recorder[1:]
		}
		sc.recorder = append(sc.recorder, currentSpeed)
		var total int64
		for _, speed := range sc.recorder {
			total += speed
		}
		if len(sc.recorder) > 0 {
			sc.NowSpeed = total / int64(len(sc.recorder))
		}
		sc.Downloaded = 0
		sc.lastResetTime = time.Now()
	}
}
