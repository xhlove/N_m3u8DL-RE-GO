package util

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"N_m3u8DL-RE-GO/internal/entity"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InteractiveSelector 交互式选择器
type InteractiveSelector struct {
	title           string
	instructions    string
	groups          []SelectionGroup
	allChoices      []*entity.StreamSpec
	selectedChoices map[int]bool
	pageSize        int
}

// SelectionGroup 选择组
type SelectionGroup struct {
	Name     string
	Choices  []*entity.StreamSpec
	StartIdx int
}

// NewInteractiveSelector 创建交互式选择器
func NewInteractiveSelector() *InteractiveSelector {
	return &InteractiveSelector{
		title:           "请选择要下载的流:",
		instructions:    "使用 ↑/↓ 方向键导航，空格键选择/取消选择，回车键确认，q 退出",
		groups:          make([]SelectionGroup, 0),
		allChoices:      make([]*entity.StreamSpec, 0),
		selectedChoices: make(map[int]bool),
		pageSize:        15,
	}
}

// SetTitle 设置标题
func (s *InteractiveSelector) SetTitle(title string) *InteractiveSelector {
	s.title = title
	return s
}

// SetInstructions 设置说明
func (s *InteractiveSelector) SetInstructions(instructions string) *InteractiveSelector {
	s.instructions = instructions
	return s
}

// SetPageSize 设置页面大小
func (s *InteractiveSelector) SetPageSize(size int) *InteractiveSelector {
	s.pageSize = size
	return s
}

// AddChoiceGroup 添加选择组
func (s *InteractiveSelector) AddChoiceGroup(groupName string, choices []*entity.StreamSpec) *InteractiveSelector {
	startIdx := len(s.allChoices)

	group := SelectionGroup{
		Name:     groupName,
		Choices:  choices,
		StartIdx: startIdx,
	}

	s.groups = append(s.groups, group)
	s.allChoices = append(s.allChoices, choices...)

	return s
}

// Select 预选择项目
func (s *InteractiveSelector) Select(choice *entity.StreamSpec) *InteractiveSelector {
	for i, c := range s.allChoices {
		if c == choice {
			s.selectedChoices[i] = true
			break
		}
	}
	return s
}

// ShowPrompt 显示交互式选择界面
func (s *InteractiveSelector) ShowPrompt() []*entity.StreamSpec {
	// 检查终端是否支持交互
	if !s.isInteractiveTerminal() {
		Logger.Warn("终端不支持交互模式，回退到简单模式")
		return s.showSimplePrompt()
	}

	// 使用Bubble Tea进行交互式选择
	return s.showBubbleTeaPrompt()
}

// isInteractiveTerminal 检查是否是交互式终端
func (s *InteractiveSelector) isInteractiveTerminal() bool {
	// 检查是否重定向
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	// 检查是否是字符设备
	return (info.Mode() & os.ModeCharDevice) != 0
}

// StreamItem 流选择项
type StreamItem struct {
	Index       int
	Stream      *entity.StreamSpec
	IsSelected  bool
	GroupName   string
	IsGroupItem bool
}

// 实现list.Item接口
func (i StreamItem) FilterValue() string {
	if i.IsGroupItem {
		return i.GroupName
	}
	return i.Stream.ToString()
}

type model struct {
	list          list.Model
	items         []StreamItem
	selectedItems map[int]bool
	delegate      *streamDelegate
	quitting      bool
	finished      bool
	result        []*entity.StreamSpec
	groups        []SelectionGroup
	lastAKeyTime  int64 // 用于检测连续按键
	aKeyCount     int   // 连续按a的次数
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case " ": // 空格键切换选择
			if selectedItem, ok := m.list.SelectedItem().(StreamItem); ok && !selectedItem.IsGroupItem {
				// 切换选择状态
				currentSelected := m.selectedItems[selectedItem.Index]
				if currentSelected {
					delete(m.selectedItems, selectedItem.Index)
				} else {
					m.selectedItems[selectedItem.Index] = true
				}

				// 获取更新后的选择状态
				newSelected := m.selectedItems[selectedItem.Index]

				// 同步更新委托中的选择状态
				m.delegate.selectedItems = m.selectedItems

				// 更新items中的选择状态
				for i := range m.items {
					if m.items[i].Index == selectedItem.Index {
						m.items[i].IsSelected = newSelected
						break
					}
				}

				// 重新设置list的items以刷新显示
				var newItems []list.Item
				for _, item := range m.items {
					newItems = append(newItems, item)
				}
				m.list.SetItems(newItems)

				// 强制重新渲染列表
				return m, nil
			}

		case "enter":
			// 完成选择
			m.finished = true
			for idx := range m.selectedItems {
				for _, item := range m.items {
					if item.Index == idx && !item.IsGroupItem {
						m.result = append(m.result, item.Stream)
						break
					}
				}
			}
			return m, tea.Quit

		case "a": // 智能选择当前分组或所有
			m.handleAKeyPress()

		case "n": // 清除当前分组选择
			m.handleNKeyPress()
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return "" // 不显示任何内容，让程序优雅退出
	}

	if m.finished {
		return "" // 不显示任何内容，让程序优雅退出
	}

	selectedCount := len(m.selectedItems)
	totalCount := 0
	for _, item := range m.items {
		if !item.IsGroupItem {
			totalCount++
		}
	}

	var header strings.Builder
	header.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("使用 ↑/↓ 方向键导航，空格键选择/取消选择，回车键确认，a选择全部，n清除选择，q退出") + "\n")
	header.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(fmt.Sprintf("已选择: %d/%d 项", selectedCount, totalCount)))

	return header.String() + m.list.View()
}

// getCurrentGroup 获取当前光标所在的分组
func (m *model) getCurrentGroup() *SelectionGroup {
	currentIndex := m.list.Index()
	if currentIndex < 0 || currentIndex >= len(m.items) {
		return nil
	}

	currentItem := m.items[currentIndex]

	// 遍历所有分组，找到当前项目所属的分组
	itemGlobalIndex := -1
	if !currentItem.IsGroupItem {
		itemGlobalIndex = currentItem.Index
	}

	for i := range m.groups {
		group := &m.groups[i]

		// 如果当前是分组标题
		if currentItem.IsGroupItem {
			// 检查分组名称是否匹配（需要通过位置判断）
			groupStartInItems := 0
			for j := 0; j < i; j++ {
				groupStartInItems += 1 + len(m.groups[j].Choices) // 1个分组标题 + N个选项
			}
			if currentIndex == groupStartInItems {
				return group
			}
		} else {
			// 如果当前是流项目，检查是否属于这个分组
			if itemGlobalIndex >= group.StartIdx && itemGlobalIndex < group.StartIdx+len(group.Choices) {
				return group
			}
		}
	}

	return nil
}

// handleAKeyPress 处理A键按下（智能选择）
func (m *model) handleAKeyPress() {
	currentTime := time.Now().UnixNano() / 1e6 // 转换为毫秒

	// 检测是否为连续按键（500ms内）
	if currentTime-m.lastAKeyTime < 500 {
		m.aKeyCount++
	} else {
		m.aKeyCount = 1
	}
	m.lastAKeyTime = currentTime

	currentGroup := m.getCurrentGroup()
	if currentGroup == nil {
		return
	}

	if m.aKeyCount >= 3 {
		// 连续按A 3次或更多：全选所有分组
		allSelected := m.areAllItemsSelected()
		for i := range m.items {
			if !m.items[i].IsGroupItem {
				if allSelected {
					delete(m.selectedItems, m.items[i].Index)
					m.items[i].IsSelected = false
				} else {
					m.selectedItems[m.items[i].Index] = true
					m.items[i].IsSelected = true
				}
			}
		}
	} else {
		// 第1-2次按A：处理当前分组
		groupSelected := m.isGroupFullySelected(currentGroup)
		for i := currentGroup.StartIdx; i < currentGroup.StartIdx+len(currentGroup.Choices); i++ {
			if groupSelected {
				delete(m.selectedItems, i)
			} else {
				m.selectedItems[i] = true
			}
		}

		// 更新items中的选择状态
		for i := range m.items {
			if !m.items[i].IsGroupItem {
				globalIndex := m.items[i].Index
				if globalIndex >= currentGroup.StartIdx && globalIndex < currentGroup.StartIdx+len(currentGroup.Choices) {
					m.items[i].IsSelected = m.selectedItems[globalIndex]
				}
			}
		}
	}

	// 同步更新并刷新显示
	m.delegate.selectedItems = m.selectedItems
	m.refreshList()
}

// handleNKeyPress 处理N键按下（清除当前分组选择）
func (m *model) handleNKeyPress() {
	currentGroup := m.getCurrentGroup()
	if currentGroup == nil {
		return
	}

	// 清除当前分组的所有选择
	for i := currentGroup.StartIdx; i < currentGroup.StartIdx+len(currentGroup.Choices); i++ {
		delete(m.selectedItems, i)
	}

	// 更新items中的选择状态
	for i := range m.items {
		if !m.items[i].IsGroupItem {
			globalIndex := m.items[i].Index
			if globalIndex >= currentGroup.StartIdx && globalIndex < currentGroup.StartIdx+len(currentGroup.Choices) {
				m.items[i].IsSelected = false
			}
		}
	}

	// 同步更新并刷新显示
	m.delegate.selectedItems = m.selectedItems
	m.refreshList()
}

// isGroupFullySelected 检查分组是否完全被选中
func (m *model) isGroupFullySelected(group *SelectionGroup) bool {
	for i := group.StartIdx; i < group.StartIdx+len(group.Choices); i++ {
		if !m.selectedItems[i] {
			return false
		}
	}
	return true
}

// areAllItemsSelected 检查是否所有流项目都被选中
func (m *model) areAllItemsSelected() bool {
	for i := range m.items {
		if !m.items[i].IsGroupItem && !m.selectedItems[m.items[i].Index] {
			return false
		}
	}
	return true
}

// refreshList 刷新列表显示
func (m *model) refreshList() {
	var newItems []list.Item
	for _, item := range m.items {
		newItems = append(newItems, item)
	}
	m.list.SetItems(newItems)
}

// showBubbleTeaPrompt 使用Bubble Tea显示交互式选择界面
func (s *InteractiveSelector) showBubbleTeaPrompt() []*entity.StreamSpec {
	// 构建项目列表
	var items []list.Item
	var streamItems []StreamItem

	for _, group := range s.groups {
		// 添加组标题
		groupItem := StreamItem{
			GroupName:   group.Name,
			IsGroupItem: true,
		}
		items = append(items, groupItem)
		streamItems = append(streamItems, groupItem)

		// 添加组中的流
		for i, stream := range group.Choices {
			globalIndex := group.StartIdx + i
			isSelected := s.selectedChoices[globalIndex]

			streamItem := StreamItem{
				Index:       globalIndex,
				Stream:      stream,
				IsSelected:  isSelected,
				IsGroupItem: false,
			}
			items = append(items, streamItem)
			streamItems = append(streamItems, streamItem)
		}
	}

	// 初始化选择状态
	selectedItems := make(map[int]bool)
	for idx, selected := range s.selectedChoices {
		if selected {
			selectedItems[idx] = true
		}
	}

	// 创建委托，传递选择状态
	delegate := &streamDelegate{
		selectedItems: selectedItems,
	}

	// 创建选择列表
	l := list.New(items, delegate, 80, 20)
	l.Title = ""              // 清空标题避免重复显示
	l.SetShowStatusBar(false) // 禁用状态栏减少空间占用
	l.SetShowHelp(false)      // 禁用底部帮助文本
	l.SetFilteringEnabled(false)

	// 同步streamItems中的IsSelected状态
	for i := range streamItems {
		if !streamItems[i].IsGroupItem {
			streamItems[i].IsSelected = selectedItems[streamItems[i].Index]
		}
	}

	m := model{
		list:          l,
		items:         streamItems,
		selectedItems: selectedItems,
		delegate:      delegate,
		groups:        s.groups,
		lastAKeyTime:  0,
		aKeyCount:     0,
	}

	// 运行程序
	program := tea.NewProgram(m)
	final, err := program.Run()
	if err != nil {
		Logger.Warn(fmt.Sprintf("交互式选择失败: %s，回退到简单模式", err.Error()))
		return s.showSimplePrompt()
	}

	if finalModel, ok := final.(model); ok {
		if finalModel.quitting {
			os.Exit(0) // 直接退出，不输出任何日志
		}
		return finalModel.result
	}

	return s.showSimplePrompt()
}

// streamDelegate 自定义列表项渲染器
type streamDelegate struct {
	selectedItems map[int]bool
}

func (d streamDelegate) Height() int                             { return 1 }
func (d streamDelegate) Spacing() int                            { return 0 }
func (d streamDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d streamDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(StreamItem)
	if !ok {
		return
	}

	isCurrentlySelected := index == m.Index()

	if item.IsGroupItem {
		// 渲染组标题
		title := fmt.Sprintf("=== %s ===", item.GroupName)
		if isCurrentlySelected {
			title = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render(title)
		} else {
			title = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(title)
		}
		fmt.Fprint(w, title)
	} else {
		// 渲染流项目 - 从委托的selectedItems获取最新状态
		isItemSelected := d.selectedItems[item.Index]

		checkbox := "○" // 未选中
		if isItemSelected {
			checkbox = "●" // 已选中
		}

		streamText := fmt.Sprintf("[%d] %s %s", item.Index, checkbox, item.Stream.ToShortString())

		if isCurrentlySelected {
			// 当前光标选中的项目
			if isItemSelected {
				streamText = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true).Render("▶ " + streamText)
			} else {
				streamText = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true).Render("▶ " + streamText)
			}
		} else if isItemSelected {
			// 已勾选但非当前光标的项目
			streamText = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("  " + streamText)
		} else {
			// 未勾选且非当前光标的项目
			streamText = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("  " + streamText)
		}

		fmt.Fprint(w, streamText)
	}
}

// showSimplePrompt 显示简单选择界面（回退方案）
func (s *InteractiveSelector) showSimplePrompt() []*entity.StreamSpec {
	fmt.Println()
	Console.MarkupLine(fmt.Sprintf("[cyan]%s[/]", s.title))
	fmt.Println()

	// 显示所有组
	choiceIndex := 0
	for _, group := range s.groups {
		Console.MarkupLine(fmt.Sprintf("[yellow]=== %s ===[/]", group.Name))

		for _, choice := range group.Choices {
			selectedMark := " "
			if s.selectedChoices[choiceIndex] {
				selectedMark = Console.Colorize("✓", Green)
			}

			fmt.Printf("[%d] %s %s\n", choiceIndex, selectedMark, choice.ToString())
			choiceIndex++
		}
		fmt.Println()
	}

	fmt.Println()
	Console.MarkupLine("[grey]输入选择 (用逗号分隔多个选择，例如: 0,1,3):[/]")
	Console.MarkupLine("[grey]或输入 'all' 选择所有，'none' 清除所有选择[/]")
	fmt.Print("选择: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	return s.processInput(input)
}

// processInput 处理简单模式的用户输入
func (s *InteractiveSelector) processInput(input string) []*entity.StreamSpec {
	// 清除所有选择
	s.selectedChoices = make(map[int]bool)

	if input == "all" {
		// 选择所有
		for i := range s.allChoices {
			s.selectedChoices[i] = true
		}
	} else if input == "none" || input == "" {
		// 不选择任何
	} else {
		// 解析用户输入的索引
		parts := strings.Split(input, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if index, err := strconv.Atoi(part); err == nil && index >= 0 && index < len(s.allChoices) {
				s.selectedChoices[index] = true
			}
		}
	}

	return s.getSelectedChoices()
}

// getSelectedChoices 获取选中的项目
func (s *InteractiveSelector) getSelectedChoices() []*entity.StreamSpec {
	var selected []*entity.StreamSpec
	for i, choice := range s.allChoices {
		if s.selectedChoices[i] {
			selected = append(selected, choice)
		}
	}
	return selected
}

// getBestStream 获取最佳流（按照C#版本的逻辑：最高带宽）
func getBestStream(streams []*entity.StreamSpec) *entity.StreamSpec {
	if len(streams) == 0 {
		return nil
	}

	// 按带宽降序排序，选择最高带宽的流
	var best *entity.StreamSpec
	var maxBandwidth int = -1

	for _, stream := range streams {
		bandwidth := 0
		if stream.Bandwidth != nil {
			bandwidth = *stream.Bandwidth
		}
		if bandwidth > maxBandwidth {
			maxBandwidth = bandwidth
			best = stream
		}
	}

	// 如果没有带宽信息，返回第一个
	if best == nil {
		best = streams[0]
	}

	return best
}

// SelectStreamsInteractive 交互式流选择主函数
func SelectStreamsInteractive(streams []*entity.StreamSpec) []*entity.StreamSpec {
	if len(streams) == 1 {
		return streams
	}

	// 分类流
	var basicStreams, audioStreams, subtitleStreams []*entity.StreamSpec

	for _, stream := range streams {
		if stream.MediaType == nil || *stream.MediaType == entity.MediaTypeVideo {
			basicStreams = append(basicStreams, stream)
		} else if *stream.MediaType == entity.MediaTypeAudio {
			audioStreams = append(audioStreams, stream)
		} else if *stream.MediaType == entity.MediaTypeSubtitles {
			subtitleStreams = append(subtitleStreams, stream)
		}
	}

	// 对各类流进行排序（按照C#的逻辑：OrderBy(MediaType).ThenByDescending(Bandwidth).ThenByDescending(GetOrder)）
	basicStreams = SortStreams(basicStreams)
	audioStreams = SortStreams(audioStreams)
	subtitleStreams = SortStreams(subtitleStreams)

	selector := NewInteractiveSelector().
		SetTitle("请选择要下载的流:").
		SetInstructions("使用 ↑/↓ 方向键导航，空格键选择/取消选择，回车键确认，a选择全部，n清除选择，q退出").
		SetPageSize(15)

	// 按照C#版本的逻辑：默认选择最佳流
	var first *entity.StreamSpec

	// 添加视频流组
	if len(basicStreams) > 0 {
		selector.AddChoiceGroup("视频流 (Video)", basicStreams)
		// 默认选择最佳基本流（最高带宽）
		best := getBestStream(basicStreams)
		if best != nil {
			selector.Select(best)
			first = best
		}
	}

	// 添加音频流组
	if len(audioStreams) > 0 {
		selector.AddChoiceGroup("音频流 (Audio)", audioStreams)

		// 默认选择相关的音频轨，或者最佳音频流
		if first != nil && first.AudioID != "" {
			// 找到与视频流关联的音频轨
			for _, audio := range audioStreams {
				if audio.GroupID == first.AudioID {
					selector.Select(audio)
					break
				}
			}
		} else {
			// 如果没有关联的音频轨，选择最佳音频流
			best := getBestStream(audioStreams)
			if best != nil {
				selector.Select(best)
			}
		}
	}

	// 添加字幕流组
	if len(subtitleStreams) > 0 {
		selector.AddChoiceGroup("字幕流 (Subtitle)", subtitleStreams)

		// 默认选择与视频流关联的字幕流，如果没有关联则全选字幕
		if first != nil && first.SubtitleID != "" {
			// 找到与视频流关联的字幕轨
			found := false
			for _, subtitle := range subtitleStreams {
				if subtitle.GroupID == first.SubtitleID {
					selector.Select(subtitle)
					found = true
					break
				}
			}
			// 如果没找到关联的字幕，则全选
			if !found {
				for _, subtitle := range subtitleStreams {
					selector.Select(subtitle)
				}
			}
		} else {
			// 按照C#版本逻辑：默认选择所有字幕
			for _, subtitle := range subtitleStreams {
				selector.Select(subtitle)
			}
		}
	}

	// 显示选择界面
	selected := selector.ShowPrompt()

	if len(selected) == 0 {
		Logger.Warn("没有选择任何流，将使用默认选择")
		// 默认选择：最佳基本流 + 最佳音频 + 所有字幕
		if len(basicStreams) > 0 {
			best := getBestStream(basicStreams)
			if best != nil {
				selected = append(selected, best)
			}
		}
		if len(audioStreams) > 0 {
			best := getBestStream(audioStreams)
			if best != nil {
				selected = append(selected, best)
			}
		}
		// 默认选择所有字幕
		selected = append(selected, subtitleStreams...)
	}

	return selected
}
