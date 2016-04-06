package collector_shared

import(
    "strings"
    "strconv"
    "github.com/golang/glog"
    "gopkg.in/mgo.v2"
)

var serverVersion string = "unknown"

func GetServerVersion(session *mgo.Session) (string) {
    if serverVersion == "unknown" {
        buildInfo, err := session.BuildInfo()
        if err != nil {
            glog.Error("Could not get BuildInfo!")
            return "unknown"
        }
        serverVersion = buildInfo.Version
        return serverVersion
    } else {
        return serverVersion
    }
}

func IsVersionGreater(version string, major int, minor int, release int) (bool) {
    split := strings.Split(version, ".")
    cmp_major, _ := strconv.Atoi(split[0])
    cmp_minor, _ := strconv.Atoi(split[1])
    cmp_release, _ := strconv.Atoi(split[2])

    if cmp_major >= major && cmp_minor >= minor && cmp_release >= release {
        return true
    }

    return false
}

func IsMyVersionGreater(session *mgo.Session, major int, minor int, release int) (bool) {
    version := GetServerVersion(session)
    return IsVersionGreater(version, major, minor, release)
}
