package conf

var SetVersion, GetVersion = create[Version]()

type Version struct {
	Value string
}
