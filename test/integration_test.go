package test

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/TykTechnologies/tyk-pump/config"
	"github.com/TykTechnologies/tyk-pump/logger"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"gopkg.in/yaml.v3"
)

var log = logger.GetLogger()

type UseCases struct {
	Name    string
	Usecase []UseCase `yaml:"usecase"`
}

type UseCase struct {
	Name  string
	Input struct {
		Config  []string `yaml:"config"`
		Records []string `yaml:"records"`
	} `yaml:"input"`
	Validate struct {
		Error    bool   `yaml:"error"`
		ErrorMsg string `yaml:"error_msg"`
		Query    struct {
			Type      string `yaml:"type"`
			OrgID     string `yaml:"org_id"`
			Timestamp string `yaml:"timestamp"`
		} `yaml:"query"`
		Result struct {
			JSONResult string `yaml:"json_result"`
		} `yaml:"result"`
	} `yaml:"validate"`
}

func TestIntegrations(t *testing.T) {
	ucs := readUseCases()

	for _, uc := range ucs.Usecase {
		t.Run(ucs.Name+"-"+uc.Name, func(t *testing.T) {
			cfg := processConfig(uc.Input.Config)
			defer postprocessEnvs(uc.Input.Config)

			pmps := []pumps.Pump{}
			for _, pumpData := range cfg.Pumps {
				pmpType, err := pumps.GetPumpByName(pumpData.Type)
				if err != nil {
					log.Error(err)
					continue
				}
				thisPmp := pmpType.New()
				thisPmp.SetFilters(pumpData.Filters)
				thisPmp.SetTimeout(pumpData.Timeout)
				thisPmp.SetOmitDetailedRecording(pumpData.OmitDetailedRecording)
				thisPmp.SetMaxRecordSize(pumpData.MaxRecordSize)
				initErr := thisPmp.Init(pumpData.Meta)
				if initErr != nil {
					log.WithField("pump", thisPmp.GetName()).Error("Pump init error (skipping): ", initErr)
				} else {
					pmps = append(pmps, thisPmp)
				}
			}

			fmt.Println(cfg.Pumps)
			t.Fail()
		})
	}
}

func readUseCases() UseCases {
	ucs := UseCases{}
	yfile, err := ioutil.ReadFile("mongo_aggregate.yaml")
	if err != nil {
		log.Fatal(err)
	}

	err2 := yaml.Unmarshal(yfile, &ucs)

	if err2 != nil {
		log.Fatal(err2)
	}
	ucs.Name = "mongo_aggregate.yaml"

	return ucs
}

func processConfig(envs []string) config.TykPumpConfiguration {
	cfg := config.TykPumpConfiguration{}

	for key, value := range getEnvsFromString(envs) {
		log.Info("setting env:", key, "=", value)
		err := os.Setenv(key, value)
		if err != nil {
			log.Error()
		}
	}

	err := cfg.LoadPumpsByEnv()
	if err != nil {
		log.Error("Error loading pumps by envs")
	}

	return cfg
}

func getEnvsFromString(envs []string) map[string]string {
	envMap := make(map[string]string)

	for _, env := range envs {
		res := strings.Split(env, "=")
		if len(res) < 2 {
			log.Error("Error parsing env:", env)
			continue
		}
		envMap[res[0]] = res[1]
	}
	return envMap
}

func postprocessEnvs(envs []string) {
	for key, _ := range getEnvsFromString(envs) {
		log.Info("unseting:", key)
		os.Unsetenv(key)
	}
}
