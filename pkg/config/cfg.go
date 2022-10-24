package config

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/muesli/termenv"
	"github.com/spf13/viper"
)

const ExampleCfgFileHeader = `## commit_convention.yml
## omit the commit_types to use the default angular-style commit types`
const ExampleCfgFileCommitTypes = `
# commit_types:
#   - type: description of what the short-form "type" means`
const ExampleCfgFileScopes = `
# scopes:
#   - scope: description of what the short-form "scope" represents`
const ExampleCfgFile = ExampleCfgFileHeader + ExampleCfgFileCommitTypes + ExampleCfgFileScopes

var (
	// see https://github.com/angular/angular.js/blob/master/DEVELOPERS.md#type
	// see https://github.com/conventional-changelog/commitlint/blob/master/%40commitlint/config-conventional/index.js#L23
	AngularPresetCommitTypes = []map[string]string{
		{"feat": "adds a new feature"},
		{"fix": "fixes a bug"},
		{"docs": "changes only the documentation"},
		{"style": "changes the style but not the meaning of the code (such as formatting)"},
		{"perf": "improves performance"},
		{"test": "adds or corrects tests"},
		{"build": "changes the build system or external dependencies"},
		{"chore": "changes outside the code, docs, or tests"},
		{"ci": "changes to the Continuous Integration (CI) system"},
		{"refactor": "changes the code without changing behavior"},
		{"revert": "reverts prior changes"},
	}
	CentralStore *viper.Viper
)

const (
	HelpSubmit = "submit: tab/enter"
	HelpBack   = "go back: shift+tab"
	HelpCancel = "cancel: ctrl+c"
	HelpSelect = "navigate: up/down"
)

func Faint(s string) string {
	return termenv.String(s).Faint().String()
}

type Cfg struct {
	CommitTypes     []map[string]string `mapstructure:"commit_types"`
	Scopes          []map[string]string `mapstructure:"scopes"`
	HeaderMaxLength int                 `mapstructure:"header_max_length"`
	//^ named similar to conventional-changelog/commitlint
	EnforceMaxLength bool `mapstructure:"enforce_header_max_length"`
}

// viper: need to deserialize YAML commit-type options
// viper: need to deserialize YAML scope options
func Init() *viper.Viper {
	CentralStore = viper.New()
	CentralStore.SetConfigName("commit_convention")
	CentralStore.SetConfigType("yaml")
	cwd, _ := filepath.Abs(".")
	// HACK: walk upwards in search of configuration rather than using the root
	// of the git repo
	paths := strings.Split(cwd, string(filepath.Separator))
	for i := 0; i < len(paths); i++ {
		path := string(os.PathSeparator) + filepath.Join(paths[0:len(paths)-i]...)
		CentralStore.AddConfigPath(path)
	}

	CentralStore.AddConfigPath("$HOME")

	CentralStore.SetDefault("commit_types", AngularPresetCommitTypes)
	CentralStore.SetDefault("scopes", map[string]string{})
	CentralStore.SetDefault("header_max_length", 72)
	CentralStore.SetDefault("enforce_header_max_length", false)
	// s.t. `git log --oneline` should remain within 80 columns w/ a 7-rune
	// commit hash and one space before the commit message.
	// this caps the max len of the `type(scope): description`, not the body
	// TODO: use env vars?

	return CentralStore
}

func Lookup(cfg *viper.Viper) Cfg {
	err := cfg.ReadInConfig()
	if err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError:
			// can fail safely, we have defaults
			break
		default:
			log.Fatal(err)
		}
	}
	var data Cfg
	err = cfg.Unmarshal(&data)
	if err != nil {
		log.Fatal(err)
	}
	return data
}
func stdoutFrom(args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out.String(), err
}

func getGitVar(var_name string) (string, error) {
	out, err := stdoutFrom("git", "var", var_name)
	if err != nil {
		return "", err
	} else {
		return strings.TrimRight(out, " \t\r\n"), err
	}
}

func GetEditor() string {
	editor := os.Getenv("EDITOR")
	if editor != "" {
		return editor
	}
	fbEditor := "vi"
	_, err := exec.LookPath(fbEditor)
	if err != nil {
		msg := "unable to open the fallback editor"
		hint := "hint: set the env variable EDITOR or install vi"
		log.Fatalf(fmt.Sprintf("%s: %q\n%s", msg, fbEditor, hint))
	}
	return fbEditor
}

// search GIT_EDITOR, then fall back to $EDITOR
func GetGitEditor() string {
	editor, err := getGitVar("GIT_EDITOR") // TODO: shell-split the string
	if err != nil {
		return GetEditor()
	}
	return editor
}

func GetCommitMessageFile() string {
	out, err := stdoutFrom("git", "rev-parse", "--absolute-git-dir")
	if err != nil {
		log.Fatal(err)
	}
	return strings.Join(
		[]string{strings.TrimRight(out, " \t\r\n"), "COMMIT_EDITMSG"},
		string(os.PathSeparator),
	)
}

func GetTerminal() string {
	terminal := os.Getenv("GITCC_TERMINAL")
	if terminal != "" {
		return terminal
	}
	fbTerminal := "xterm"
	_, err := exec.LookPath(fbTerminal)
	if err != nil {
		msg := "unable to open the fallback terminal"
		hint := "hint: set the env variable GITCC_TERMINAL or install xterm"
		log.Fatalf(fmt.Sprintf("%s: %q\n%s", msg, fbTerminal, hint))
	}
	return fbTerminal
}

// interactively edit the config file, if any was used.
func EditCfgFile(cfg *viper.Viper, defaultFileContent string) Cfg {
	cfgFile := cfg.ConfigFileUsed()
	if cfgFile == "" {
		cfgFile = "commit_convention.yml" // TODO: verify that this is the correct location (i.e. the cwd or a parent directory)?
		f, err := os.Create(cfgFile)
		if err != nil {
			log.Fatalf("unable to create file %s: %+v", cfgFile, err)
		}
		_, err = f.WriteString(fmt.Sprintf(defaultFileContent))
		if err != nil {
			log.Fatalf("unable to write to file: %v", err)
		}
	}
	cmd := exec.Command(GetTerminal(), "-e", GetEditor()+" "+cfgFile)
	fmt.Println(cmd)
	cmd.Stdin, cmd.Stdout = os.Stdin, os.Stderr
	cmd.Run() // ignore errors
	return Lookup(cfg)
}
