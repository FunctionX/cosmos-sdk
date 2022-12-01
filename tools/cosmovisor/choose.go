package cosmovisor

import (
	"encoding/json"
	"fmt"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/store"
	tmtypes "github.com/tendermint/tendermint/types"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
)

const (
	EnvChooseVersion     = "DAEMON_CHOOSE_VERSION"
	EnvCosmovisorReplace = "DAEMON_COSMOVISOR_REPLACE"
	EnvBinaryPath        = "DAEMON_BINARY_PATH"
	EnvUpgradesInfoPath  = "DAEMON_UPGRADES_INFO_PATH"
)

func ReplaceCosmovisor(logger *zerolog.Logger) error {
	replaceFlag, err := booleanOption(EnvCosmovisorReplace, false)
	if err != nil {
		return err
	}
	logger.Info().Bool("replace-flag", replaceFlag).Msg("replace cosmovisor")
	if !replaceFlag {
		return nil
	}
	// check home directory
	homeDir := os.Getenv(EnvHome)
	if !filepath.IsAbs(homeDir) {
		return fmt.Errorf("%s must be an absolute path", EnvHome)
	}
	if err := checkDir(homeDir); err != nil {
		return err
	}

	// check binary directory
	binaryPath := os.Getenv(EnvBinaryPath)
	if !filepath.IsAbs(binaryPath) {
		return fmt.Errorf("%s must be an absolute path", EnvBinaryPath)
	}
	if err := checkDir(binaryPath); err != nil {
		return err
	}

	// get chain id
	chainID, err := getChainId(homeDir)
	if err != nil {
		return err
	}

	//check upgrades-info directory
	upgradesInfoPath := os.Getenv(EnvUpgradesInfoPath)
	chainUpgradesInfoPath := filepath.Join(upgradesInfoPath, chainID)
	if !filepath.IsAbs(chainUpgradesInfoPath) {
		return fmt.Errorf("%s must be an absolute path", EnvUpgradesInfoPath)
	}
	if err := checkDir(chainUpgradesInfoPath); err != nil {
		return err
	}

	cosmovisorPath := filepath.Join(homeDir, rootName)

	// remove default cosmovisor
	logger.Info().Str("path", cosmovisorPath).Msg("remove default cosmovisor")
	os.RemoveAll(cosmovisorPath)

	//copy binary to default cosmovisor
	logger.Info().Str("src", binaryPath).Str("dest", cosmovisorPath).Msg("copy binary")
	if err := copyDir(binaryPath, cosmovisorPath); err != nil {
		return fmt.Errorf("copy binary error %s", err.Error())
	}

	//copy upgrades-info to default cosmovisor
	logger.Info().Str("src", chainUpgradesInfoPath).Str("dest", cosmovisorPath).Msg("copy upgrades-info")
	if err := copyDir(chainUpgradesInfoPath, cosmovisorPath); err != nil {
		return fmt.Errorf("copy upgrades-info error %s", err)
	}

	return nil
}

func RemoveCosmovisor(logger *zerolog.Logger) error {
	replaceFlag, err := booleanOption(EnvCosmovisorReplace, false)
	if err != nil {
		return err
	}
	if !replaceFlag {
		return nil
	}

	// check home directory
	homeDir := os.Getenv(EnvHome)
	if !filepath.IsAbs(homeDir) {
		return fmt.Errorf("%s must be an absolute path", EnvHome)
	}
	if err := checkDir(homeDir); err != nil {
		return err
	}

	// remove default cosmovisor
	cosmovisorPath := filepath.Join(homeDir, rootName)

	logger.Info().Str("path", cosmovisorPath).Msg("remove default cosmovisor")
	return os.RemoveAll(cosmovisorPath)
}

func copyFile(src, dst string) error {
	var err error
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo

	if srcfd, err = os.Open(src); err != nil {
		return err
	}
	defer srcfd.Close()

	if dstfd, err = os.Create(dst); err != nil {
		return err
	}
	defer dstfd.Close()

	if _, err = io.Copy(dstfd, srcfd); err != nil {
		return err
	}
	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}
	return os.Chmod(dst, srcinfo.Mode())
}

func copyDir(src string, dst string) error {
	var err error
	var fds []os.FileInfo
	var srcinfo os.FileInfo

	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcinfo.Mode()); err != nil {
		return err
	}

	des, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	fds = make([]fs.FileInfo, 0, len(des))
	for _, de := range des {
		info, err := de.Info()
		if err != nil {
			return err
		}
		fds = append(fds, info)
	}

	for _, fd := range fds {
		srcfp := path.Join(src, fd.Name())
		dstfp := path.Join(dst, fd.Name())

		if fd.IsDir() {
			if err = copyDir(srcfp, dstfp); err != nil {
				return err
			}
		} else {
			if err = copyFile(srcfp, dstfp); err != nil {
				return err
			}
		}
	}
	return nil
}

func getChainId(homeDir string) (chainId string, err error) {
	tmCfg, err := readTmCfg(homeDir)
	if err != nil {
		return "", err
	}
	genesis, err := tmtypes.GenesisDocFromFile(tmCfg.GenesisFile())
	if err != nil {
		return "", err
	}

	return genesis.ChainID, nil
}

func checkDir(p string) error {
	stat, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("invalid %s: %s", p, err)
	}
	if !stat.IsDir() {
		return fmt.Errorf("%s must be a directory", p)
	}
	return nil
}

func ChooseVersion(logger *zerolog.Logger, cfg *Config) error {
	chooseVersion, err := booleanOption(EnvChooseVersion, false)
	if err != nil {
		return err
	}
	if !chooseVersion {
		return nil
	}

	plans, err := allUpgrades(logger, cfg)
	if err != nil {
		return err
	}

	height, err := blockStoreHeight(logger, cfg.Home)
	if err != nil {
		return err
	}

	plan := choosePlan(plans, height)

	logger.Info().Int64("current-height", height).Str("plan", plan.String()).Msg("choose plan")

	return symLinkToPlan(logger, cfg, plan)
}

func allUpgrades(logger *zerolog.Logger, cfg *Config) ([]upgradetypes.Plan, error) {
	var plans = make([]upgradetypes.Plan, 0, 10)
	var planExist = make(map[int64]bool, 10)

	//cosmovisor upgrades
	upgradePath := cfg.BaseUpgradeDir()
	upgradeDirStat, err := os.Stat(upgradePath)
	if err != nil {
		return nil, fmt.Errorf("cannot stat dir %s: %w", upgradePath, err)
	}
	if !upgradeDirStat.IsDir() {
		return nil, fmt.Errorf("upgrades path %s not dir", upgradePath)
	}
	dirFile, err := os.ReadDir(upgradePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read dir %s: %w", upgradePath, err)
	}
	for _, fi := range dirFile {
		if !fi.IsDir() {
			logger.Warn().Str("version", fi.Name()).Str("path", filepath.Join(upgradePath, fi.Name())).Msg("invalid upgrades")
			continue
		}
		//check upgrade bin
		upgradeBinPath := cfg.UpgradeBin(fi.Name())
		if err := EnsureBinary(upgradeBinPath); err != nil {
			return nil, err
		}
		//check upgrade info
		upgradeInfoFilePath := filepath.Join(upgradePath, fi.Name(), upgradetypes.UpgradeInfoFilename)
		upgradeInfoStat, err := os.Stat(upgradeInfoFilePath)
		if err != nil {
			logger.Info().Str("version", fi.Name()).Msg("upgrade-info.json not exist(maybe next upgrade)")
			continue
		}
		if upgradeInfoStat.IsDir() {
			return nil, fmt.Errorf("upgrade-info path %s is dir", upgradeInfoFilePath)
		}
		// parse upgrade-info file
		info, err := parseUpgradeInfo(upgradeInfoFilePath)
		if err != nil {
			return nil, err
		}
		if !planExist[info.Height] {
			logger.Info().Int64("upgrade-height", info.Height).Str("name", info.Name).Msg("add upgrade plan")
			plans = append(plans, info)
			planExist[info.Height] = true
		}
	}

	//data upgrade-info
	dataUpgradeInfoFilePath := cfg.UpgradeInfoFilePath()
	if dataUpgradeInfoStat, err := os.Stat(dataUpgradeInfoFilePath); err == nil {
		if dataUpgradeInfoStat.IsDir() {
			return nil, fmt.Errorf("upgrade-info path %s is dir", dataUpgradeInfoFilePath)
		}
		// parse upgrade-info file
		info, err := parseUpgradeInfo(dataUpgradeInfoFilePath)
		if err != nil {
			return nil, err
		}
		if !planExist[info.Height] {
			logger.Info().Int64("upgrade-height", info.Height).Str("name", info.Name).Msg("add upgrade plan")
			plans = append(plans, info)
			planExist[info.Height] = true

			upgradeBinPath := cfg.UpgradeBin(info.Name)
			if err := EnsureBinary(upgradeBinPath); err != nil {
				return nil, fmt.Errorf("binary not found for %s plan", info.Name)
			}
		}
	}

	//reverse sort all upgrade by height
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].Height > plans[j].Height
	})
	return plans, nil
}

func parseUpgradeInfo(upgradeInfoFilePath string) (upgradetypes.Plan, error) {
	bz, err := os.ReadFile(upgradeInfoFilePath)
	if err != nil {
		return upgradetypes.Plan{}, fmt.Errorf("can not read file %s: %w", upgradeInfoFilePath, err)
	}
	//{"name":"fxv2","height":3418880}
	upgradeInfo := upgradetypes.Plan{}
	if err = json.Unmarshal(bz, &upgradeInfo); err != nil {
		return upgradetypes.Plan{}, fmt.Errorf("can not unmarshal file %s: %w", upgradeInfoFilePath, err)
	}
	return upgradeInfo, nil
}

func readTmCfg(homeDir string) (*config.Config, error) {
	configPath := filepath.Join(homeDir, "config")
	tmCfgFile := filepath.Join(configPath, "config.toml")

	rootViper := viper.New()
	rootViper.SetConfigType("toml")
	rootViper.SetConfigName("config")
	rootViper.AddConfigPath(configPath)

	if err := rootViper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read in %s: %w", tmCfgFile, err)
	}

	conf := config.DefaultConfig()
	if err := rootViper.Unmarshal(conf); err != nil {
		return nil, err
	}
	conf.SetRoot(homeDir)

	return conf, nil
}

func blockStoreHeight(logger *zerolog.Logger, homeDir string) (int64, error) {
	tmCfg, err := readTmCfg(homeDir)
	if err != nil {
		return 0, err
	}

	logger.Info().Str("home", tmCfg.RootDir).Str("db_backend", tmCfg.DBBackend).
		Str("db_path", tmCfg.DBPath).Msg("block store height")

	blockStoreDB, err := node.DefaultDBProvider(&node.DBContext{ID: "blockstore", Config: tmCfg})
	if err != nil {
		return 0, fmt.Errorf("db provider %s: %w", tmCfg.DBDir(), err)
	}
	defer blockStoreDB.Close()
	blockStore := store.NewBlockStore(blockStoreDB)
	return blockStore.Height(), nil
}

func choosePlan(plans []upgradetypes.Plan, height int64) upgradetypes.Plan {
	currentPlan := upgradetypes.Plan{Name: genesisDir, Height: 0}
	for _, p := range plans {
		if height >= p.Height {
			currentPlan = p
			break
		}
	}
	return currentPlan
}

func symLinkToPlan(logger *zerolog.Logger, cfg *Config, plan upgradetypes.Plan) error {
	newLink := filepath.Join(cfg.Root(), genesisDir)
	if plan.Name != genesisDir || plan.Height != 0 {
		safeName := url.PathEscape(plan.Name)
		newLink = filepath.Join(cfg.Root(), upgradesDir, safeName)
	}

	logger.Info().Str("name", plan.Name).Int64("upgrade-height", plan.Height).Str("link", newLink).Msg("symlink to bin")

	current := filepath.Join(cfg.Root(), currentLink)
	// remove link if it exists
	if _, err := os.Stat(current); err == nil {
		oldLink, _ := os.Readlink(current)
		if oldLink == newLink {
			//not need create new symlink
			logger.Info().Msg("new symlink equal to old symlink")
			return nil
		}
		logger.Info().Str("link", oldLink).Msg("remove old current link")
		os.Remove(current)
	}

	os.RemoveAll(current)

	// point to the new directory
	return os.Symlink(newLink, current)
}
