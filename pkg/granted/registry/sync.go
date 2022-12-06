package registry

import (
	"os"

	"github.com/common-fate/clio"
	"github.com/urfave/cli/v2"
)

var SyncCommand = cli.Command{
	Name:        "sync",
	Usage:       "Pull the latest change from remote origin and sync aws profiles in aws config files",
	Description: "Pull the latest change from remote origin and sync aws profiles in aws config files",
	Action: func(c *cli.Context) error {
		if err := SyncProfileRegistries(false); err != nil {
			return err
		}

		return nil
	},
}

// Wrapper around sync func. Check if profile registry is configured, pull the latest changes and call sync func.
func SyncProfileRegistries(shouldSilentLog bool) error {
	registries, err := GetProfileRegistries()
	if err != nil {
		return err
	}

	if len(registries) == 0 {
		clio.Warn("granted registry not configured. Try adding a git repository with 'granted registry add <https://github.com/your-org/your-registry.git>'")
	}

	awsConfigPath, err := getDefaultAWSConfigLocation()
	if err != nil {
		return err
	}

	configFile, err := loadAWSConfigFile()
	if err != nil {
		return err
	}

	// if the config file contains granted generated content then remove it
	if err := removeAutogeneratedProfiles(configFile, awsConfigPath); err != nil {
		return err
	}

	for index, r := range registries {
		repoDirPath, err := r.getRegistryLocation()
		if err != nil {
			return err
		}

		// If the local repo has been deleted, then attempt to clone it again
		_, err = os.Stat(repoDirPath)
		if os.IsNotExist(err) {
			err = gitClone(r.Config.URL, repoDirPath)
			if err != nil {
				return err
			}
		} else {
			err = gitPull(repoDirPath, shouldSilentLog)
			if err != nil {
				return err
			}
		}

		err = r.Parse()
		if err != nil {
			return err
		}

		isFirstSection := false
		if index == 0 {
			isFirstSection = true
		}

		if err := Sync(&r, isFirstSection, SYNC_COMMAND); err != nil {
			return err
		}
	}

	return nil
}

type CommandType string

const (
	ADD_COMMAND       CommandType = "add"
	SYNC_COMMAND      CommandType = "sync"
	AUTO_SYNC_COMMAND CommandType = "autosync"
)

// Sync function will load all the configs provided in the clonedFile.
// and generated a new section in the ~/.aws/profile file.
func Sync(r *Registry, isFirstSection bool, cmd CommandType) error {
	clio.Debugf("syncing %s \n", r.Config.Name)

	awsConfigPath, err := getDefaultAWSConfigLocation()
	if err != nil {
		return err
	}

	awsConfigFile, err := loadAWSConfigFile()
	if err != nil {
		return err
	}

	clonedFile, err := loadClonedConfigs(*r)
	if err != nil {
		return err
	}

	err = generateNewRegistrySection(r, awsConfigFile, clonedFile, isFirstSection, cmd)
	if err != nil {
		return err
	}

	err = awsConfigFile.SaveTo(awsConfigPath)
	if err != nil {
		return err
	}

	clio.Successf("Successfully synced registry %s", r.Config.Name)

	return nil
}
