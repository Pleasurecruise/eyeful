package project

const (
	Placeholder = "{test}"
	Declared    = ".github/eyeful.yml"
)

var manifests = []string{
	"go.mod", ".golangci.yml", ".golangci.yaml", ".golangci.toml", "Cargo.toml",
	"pyproject.toml", "setup.py", "setup.cfg", "tox.ini", "pytest.ini", "conftest.py", "ruff.toml", "requirements.txt", "uv.lock", "poetry.lock",
	"package.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb", "package-lock.json",
}

type Config struct {
	Setup   string `yaml:"setup"`
	Lint    string `yaml:"lint"`
	Test    string `yaml:"test"`
	TestOne string `yaml:"test_one"`
}

type Command struct {
	Name string
	Run  string
}

type manifest struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Scripts         map[string]string `json:"scripts"`
}
