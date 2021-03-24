package config

type Config struct {
	MongoInstance []*MongoInstance `yaml:"mongo-list",json:"mongo-list"`
}

type MongoInstance struct {
	Name    string     `yaml:"name",json:"name"`
	Host    string     `yaml:"host",json:"host"`
	Port    string     `yaml:"port",json:"port"`
	Account []*Account `yaml:"account",json:"account"`
}

type Account struct {
	Username string `yaml:"username",json:"username"`
	Password string `yaml:"password",json:"password"`
}
