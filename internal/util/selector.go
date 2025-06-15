package util

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"N_m3u8DL-RE-GO/internal/entity"
)

// MultiSelectionPrompt 多选择提示器，类似Spectre.Console的MultiSelectionPrompt
type MultiSelectionPrompt struct {
	title           string
	instructions    string
	moreChoicesText string
	pageSize        int
	required        bool
	groups          []ChoiceGroup
	allChoices      []*entity.StreamSpec
	selectedChoices map[int]bool
	currentPage     int
}

// ChoiceGroup 选择组
type ChoiceGroup struct {
	Name    string
	Choices []*entity.StreamSpec
}

// NewMultiSelectionPrompt 创建新的多选择提示器
func NewMultiSelectionPrompt() *MultiSelectionPrompt {
	return &MultiSelectionPrompt{
		title:           "请选择要下载的流:",
		instructions:    "使用方向键导航，空格键选择/取消选择，回车键确认",
		moreChoicesText: "(使用上下方向键查看更多选项)",
		pageSize:        10,
		required:        true,
		groups:          make([]ChoiceGroup, 0),
		allChoices:      make([]*entity.StreamSpec, 0),
		selectedChoices: make(map[int]bool),
		currentPage:     0,
	}
}

// Title 设置标题
func (p *MultiSelectionPrompt) Title(title string) *MultiSelectionPrompt {
	p.title = title
	return p
}

// InstructionsText 设置说明文字
func (p *MultiSelectionPrompt) InstructionsText(text string) *MultiSelectionPrompt {
	p.instructions = text
	return p
}

// MoreChoicesText 设置更多选择的提示文字
func (p *MultiSelectionPrompt) MoreChoicesText(text string) *MultiSelectionPrompt {
	p.moreChoicesText = text
	return p
}

// PageSize 设置页面大小
func (p *MultiSelectionPrompt) PageSize(size int) *MultiSelectionPrompt {
	p.pageSize = size
	return p
}

// Required 设置是否必须选择
func (p *MultiSelectionPrompt) Required() *MultiSelectionPrompt {
	p.required = true
	return p
}

// AddChoiceGroup 添加选择组
func (p *MultiSelectionPrompt) AddChoiceGroup(header *entity.StreamSpec, choices []*entity.StreamSpec) {
	if header.Name != "" && strings.HasPrefix(header.Name, "__") {
		groupName := header.Name[2:] // 移除前缀 "__"
		p.groups = append(p.groups, ChoiceGroup{
			Name:    groupName,
			Choices: choices,
		})
	}
	p.allChoices = append(p.allChoices, choices...)
}

// Select 预选择项目
func (p *MultiSelectionPrompt) Select(choice *entity.StreamSpec) {
	for i, c := range p.allChoices {
		if c == choice {
			p.selectedChoices[i] = true
			break
		}
	}
}

// ShowPrompt 显示选择界面（简化版本）
func (p *MultiSelectionPrompt) ShowPrompt() []*entity.StreamSpec {
	fmt.Println()
	Console.MarkupLine(fmt.Sprintf("[cyan]%s[/]", p.title))
	fmt.Println()

	// 显示所有组
	choiceIndex := 0
	for _, group := range p.groups {
		Console.MarkupLine(fmt.Sprintf("[yellow]=== %s ===[/]", group.Name))

		for _, choice := range group.Choices {
			selectedMark := " "
			if p.selectedChoices[choiceIndex] {
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

	return p.processInput(input)
}

// processInput 处理用户输入
func (p *MultiSelectionPrompt) processInput(input string) []*entity.StreamSpec {
	// 清除所有选择
	p.selectedChoices = make(map[int]bool)

	if input == "all" {
		// 选择所有
		for i := range p.allChoices {
			p.selectedChoices[i] = true
		}
	} else if input == "none" {
		// 不选择任何
	} else if input != "" {
		// 解析用户输入的索引
		parts := strings.Split(input, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if index, err := strconv.Atoi(part); err == nil && index >= 0 && index < len(p.allChoices) {
				p.selectedChoices[index] = true
			}
		}
	}

	// 如果没有选择任何项目且设置为必须选择，则默认选择第一个
	if p.required && len(p.selectedChoices) == 0 && len(p.allChoices) > 0 {
		p.selectedChoices[0] = true
	}

	// 收集选中的项目
	var selected []*entity.StreamSpec
	for i, choice := range p.allChoices {
		if p.selectedChoices[i] {
			selected = append(selected, choice)
		}
	}

	return selected
}
