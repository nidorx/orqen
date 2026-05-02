package conf

var SetInfo, GetInfo = create[Info]()

type Info struct {
	Version string
	Website string
}
