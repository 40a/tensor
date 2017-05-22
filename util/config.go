package util

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"strconv"

	"encoding/base64"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/securecookie"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"
)

var InteractiveSetup bool
var Secrets bool

type MongoDBConfig struct {
	Hosts      []string `yaml:"hosts"`
	Username   string   `yaml:"user"`
	Password   string   `yaml:"pass"`
	DbName     string   `yaml:"name"`
	ReplicaSet string   `yaml:"replica_set"`
}

type configType struct {
	MongoDB MongoDBConfig `yaml:"mongodb"`

	RabbitMQ string `yaml:"rabbitmq"`

	// TCP address to listen on, ":http" if empty
	Host string `yaml:"host"`
	Port string `yaml:"port"`

	// Tensor stores projects here
	ProjectsHome string `yaml:"projects_home"`

	// cookie hashing & encryption
	Salt string `yaml:"salt"`

	AnsibleJobTimeOut   int `yaml:"ansible_job_timeout"`
	SyncJobTimeOut      int `yaml:"sync_job_timeout"`
	TerraformJobTimeOut int `yaml:"terraform_job_timeout"`

	JWTTimeout        int `yaml:"jwt_timeout"`
	JWTRefreshTimeout int `yaml:"jwt_refresh_timeout"`

	TLSEnabled        bool   `yaml:"tls_enabled"`
	SSLCertificate    string `yaml:"ssl_certificate"`
	SSLCertificateKey string `yaml:"ssl_certificate_key"`

	Debug bool `yaml:"debug"`
}

func (c configType) GetAddress() string {
	return c.Host + ":" + c.Port
}

func (c configType) GetUrl() (address string) {
	address = "http://" + c.Host + ":" + c.Port
	if c.TLSEnabled {
		address = "https://" + c.Host + ":" + c.Port
	}
	return
}

var Config *configType

func init() {
	flag.BoolVar(&InteractiveSetup, "setup", false, "perform interactive setup")
	flag.BoolVar(&Secrets, "secrets", false, "generate salt")
	var pwd string
	flag.StringVar(&pwd, "hash", "", "generate hash of given password")

	flag.Parse()

	if len(pwd) > 0 {
		password, _ := bcrypt.GenerateFromPassword([]byte(pwd), 11)
		fmt.Println("Generated password: ", string(password))
		os.Exit(0)
	}
	if Secrets {
		GenerateSalt()
		os.Exit(0)
	}

	if _, err := os.Stat("/etc/tensor.conf"); os.IsNotExist(err) {
		logrus.Println("Configuration file does not exist")
		Config = &configType{} // initialize empty

	} else {
		conf, err := ioutil.ReadFile("/etc/tensor.conf")

		if err != nil {
			logrus.Fatal(errors.New("Could not find configuration!\n\n" + err.Error()))
			os.Exit(5)
		}

		if err := yaml.Unmarshal(conf, &Config); err != nil {
			logrus.Fatal("Invalid Configuration!\n\n" + err.Error())
			os.Exit(6)
		}
	}

	if len(os.Getenv("TENSOR_HOST")) > 0 {
		Config.Host = os.Getenv("TENSOR_HOST")
	} else if len(Config.Host) == 0 {
		Config.Host = "0.0.0.0"
	}

	if len(os.Getenv("TENSOR_PORT")) > 0 {
		Config.Host = os.Getenv("TENSOR_PORT")
	} else if len(Config.Host) == 0 {
		Config.Port = "80"
	}

	if len(os.Getenv("PROJECTS_HOME")) > 0 {
		Config.ProjectsHome = os.Getenv("PROJECTS_HOME")
	} else if len(Config.ProjectsHome) == 0 {
		Config.ProjectsHome = "/opt/tensor/projects"
	}

	if len(os.Getenv("TENSOR_SALT")) > 0 {
		Config.Salt = os.Getenv("TENSOR_SALT")
	} else if len(Config.Salt) == 0 {
		Config.Salt = "8m86pie1ef8bghbq41ru!de4"
	}

	if len(os.Getenv("TENSOR_ANSIBLE_JOB_TIMEOUT")) > 0 {
		time, _ := strconv.Atoi(os.Getenv("TENSOR_ANSIBLE_JOB_TIMEOUT"))
		Config.AnsibleJobTimeOut = time
	} else if Config.AnsibleJobTimeOut == 0 {
		Config.AnsibleJobTimeOut = 3600
	}

	if len(os.Getenv("TENSOR_TERRAFORM_JOB_TIMEOUT")) > 0 {
		time, _ := strconv.Atoi(os.Getenv("TENSOR_TERRAFORM_JOB_TIMEOUT"))
		Config.TerraformJobTimeOut = time
	} else if Config.TerraformJobTimeOut == 0 {
		Config.TerraformJobTimeOut = 3600
	}

	if len(os.Getenv("TENSOR_SYNC_JOB_TIMEOUT")) > 0 {
		time, _ := strconv.Atoi(os.Getenv("TENSOR_SYNC_JOB_TIMEOUT"))
		Config.SyncJobTimeOut = time
	} else if Config.SyncJobTimeOut == 0 {
		Config.SyncJobTimeOut = 3600
	}

	if len(os.Getenv("TENSOR_JWT_TIMEOUT")) > 0 {
		time, _ := strconv.Atoi(os.Getenv("TENSOR_JWT_TIMEOUT"))
		Config.JWTTimeout = time
	} else if Config.JWTTimeout == 0 {
		Config.JWTTimeout = 3600
	}

	if len(os.Getenv("TENSOR_JWT_REFRESH_TIMEOUT")) > 0 {
		time, _ := strconv.Atoi(os.Getenv("TENSOR_JWT_REFRESH_TIMEOUT"))
		Config.JWTRefreshTimeout = time
	} else if Config.JWTRefreshTimeout == 0 {
		Config.JWTRefreshTimeout = 3600
	}

	if len(os.Getenv("TENSOR_DB_USER")) > 0 {
		Config.MongoDB.Username = os.Getenv("TENSOR_DB_USER")
	}

	if len(os.Getenv("TENSOR_DB_PASSWORD")) > 0 {
		Config.MongoDB.Password = os.Getenv("TENSOR_DB_PASSWORD")
	}

	if len(os.Getenv("TENSOR_DB_NAME")) > 0 {
		Config.MongoDB.DbName = os.Getenv("TENSOR_DB_NAME")
	}

	if len(os.Getenv("TENSOR_DB_REPLICA")) > 0 {
		Config.MongoDB.ReplicaSet = os.Getenv("TENSOR_DB_REPLICA")
	}

	if len(os.Getenv("TENSOR_DB_HOSTS")) > 0 {
		Config.MongoDB.Hosts = strings.Split(os.Getenv("TENSOR_DB_HOSTS"), ";")
	}

	if len(os.Getenv("TENSOR_RABBITMQ")) > 0 {
		Config.RabbitMQ = os.Getenv("TENSOR_RABBITMQ")
	}

	// TLS configuration
	if os.Getenv("TENSOR_TLS_ENABLED") == "true" {
		Config.TLSEnabled = true
	}

	if len(os.Getenv("TENSOR_SSL_CERTIFICATE")) > 0 {
		Config.SSLCertificate = os.Getenv("TENSOR_SSL_CERTIFICATE")
	}

	if len(os.Getenv("TENSOR_SSL_CERTIFICATE_KEY")) > 0 {
		Config.SSLCertificateKey = os.Getenv("TENSOR_SSL_CERTIFICATE_KEY")
	}

	// Debug configuration
	if os.Getenv("TENSOR_DEBUG") == "true" {
		Config.Debug = true
	}

	if _, err := os.Stat(Config.ProjectsHome); os.IsNotExist(err) {
		fmt.Printf(" Running: mkdir -p %v..\n", Config.ProjectsHome)
		if err := os.MkdirAll(Config.ProjectsHome, 0755); err != nil {
			logrus.Fatal(err)
			os.Exit(7)
		}
	}

}

func GenerateSalt() {
	salt := securecookie.GenerateRandomKey(18)
	fmt.Println("Generated Salt: ", base64.URLEncoding.EncodeToString(salt))
}
