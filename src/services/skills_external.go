package services

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hsqbyte/qbot/src/core"
)

// frontmatterMeta 解析 SKILL.md frontmatter 得到的元数据
type frontmatterMeta struct {
	Name             string
	Description      string
	ParametersSchema string
}

// 默认的参数 schema（用于 computer-control 等通用命令技能）
var defaultSchema = `{"type":"object","properties":{"command":{"type":"string","description":"要执行的命令或操作参数"}},"required":[]}`

// LoadExternalSkills 扫描 skills/ 目录下的子目录，读取 SKILL.md 加载技能
// 遵循 agentskills.io 规范
func LoadExternalSkills() {
	skillsDir := "skills"
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(skillsDir, 0755)
			core.Log.Infof("已创建外部技能目录: %s", skillsDir)
		} else {
			core.Log.Errorf("扫描外部技能目录失败: %v", err)
		}
		return
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		skillDir := filepath.Join(skillsDir, entry.Name())
		skillMdPath := filepath.Join(skillDir, "SKILL.md")

		data, err := os.ReadFile(skillMdPath)
		if err != nil {
			core.Log.Warnf("跳过技能目录 [%s]: 未找到 SKILL.md", skillDir)
			continue
		}

		meta := parseFrontmatter(data)
		if meta.Name == "" || meta.Description == "" {
			core.Log.Warnf("跳过技能 [%s]: SKILL.md frontmatter 缺少 name 或 description", entry.Name())
			continue
		}

		// 验证 name 与目录名一致（agentskills.io 规范）
		if meta.Name != entry.Name() {
			core.Log.Warnf("技能 [%s]: SKILL.md name '%s' 与目录名不一致，使用目录名", entry.Name(), meta.Name)
			meta.Name = entry.Name()
		}

		// 如果没有自定义 schema，使用默认的
		schema := meta.ParametersSchema
		if schema == "" {
			schema = defaultSchema
		}

		// 查找 scripts/execute.py 或 scripts/execute.sh
		scriptPath := findScript(skillDir)

		// 闭包捕获
		currentPath := scriptPath
		currentName := meta.Name

		skill := Skill{
			Name:             currentName,
			Description:      meta.Description,
			ParametersSchema: schema,
		}

		if currentPath != "" {
			skill.Execute = func(args string, onProgress func(string)) string {
				core.Log.Infof("▶️ 执行脚本技能: %s args=%s", currentName, args)
				var cmd *exec.Cmd
				if strings.HasSuffix(currentPath, ".py") {
					cmd = exec.Command("python3", currentPath)
				} else {
					cmd = exec.Command(currentPath)
				}

				// 通过 stdin 传递参数，避免命令注入
				cmd.Stdin = strings.NewReader(args)

				stdoutPipe, err := cmd.StdoutPipe()
				if err != nil {
					return fmt.Sprintf("获取输出流失败: %v", err)
				}

				var stderr bytes.Buffer
				cmd.Stderr = &stderr

				if err := cmd.Start(); err != nil {
					return fmt.Sprintf("启动失败: %v, stderr: %s", err, stderr.String())
				}

				var fullOutput strings.Builder
				ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`) // 用于清理颜色控制符
				scanner := bufio.NewScanner(stdoutPipe)
				for scanner.Scan() {
					line := scanner.Text()
					fullOutput.WriteString(line + "\n")

					cleanLine := ansiRegex.ReplaceAllString(line, "")
					if onProgress != nil {
						onProgress(cleanLine)
					}
				}

				if err := cmd.Wait(); err != nil {
					return fmt.Sprintf("执行失败: %v, stderr: %s\nStdout:\n%s", err, stderr.String(), fullOutput.String())
				}
				return fullOutput.String()
			}
		} else {
			// 没有脚本，把 SKILL.md 内容作为指令提供给模型
			skill.Execute = func(args string, onProgress func(string)) string {
				core.Log.Infof("📋 加载技能指令: %s (无脚本)", currentName)
				return string(data)
			}
		}

		RegisterSkill(skill)
		count++
		core.Log.Infof("⚙️ 成功加载技能: %s (来自 %s)", currentName, skillDir)
	}

	if count > 0 {
		core.Log.Infof("🔗 技能目录扫描完成，共加载 %d 个技能", count)
	}
}

// parseFrontmatter 从 SKILL.md 内容中解析 YAML frontmatter
func parseFrontmatter(data []byte) frontmatterMeta {
	content := string(data)
	re := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---`)
	matches := re.FindStringSubmatch(content)
	if len(matches) < 2 {
		return frontmatterMeta{}
	}

	fm := matches[1]
	result := frontmatterMeta{}

	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			result.Name = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "name:")), "\"'")
		}
		if strings.HasPrefix(line, "description:") {
			result.Description = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "description:")), "\"'")
		}
		if strings.HasPrefix(line, "parameters_schema:") {
			result.ParametersSchema = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "parameters_schema:")), "\"'")
		}
	}

	// 从 metadata.parameters_schema 中提取（如果有的话）
	if result.ParametersSchema == "" {
		reMeta := regexp.MustCompile(`parameters_schema:\s*['"](.+?)['"]`)
		metaMatch := reMeta.FindStringSubmatch(fm)
		if len(metaMatch) > 1 {
			result.ParametersSchema = metaMatch[1]
		}
	}

	return result
}

// findScript 在技能目录的 scripts/ 下查找可执行脚本
func findScript(skillDir string) string {
	scriptsDir := filepath.Join(skillDir, "scripts")
	for _, name := range []string{"execute.py", "execute.sh"} {
		p := filepath.Join(scriptsDir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
