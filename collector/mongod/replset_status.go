package collector_mongod

import (
    "time"
    "github.com/golang/glog"
    "github.com/prometheus/client_golang/prometheus"
    "gopkg.in/mgo.v2"
    "gopkg.in/mgo.v2/bson"
)

var (
    replSetLastElection = prometheus.NewCounterVec(prometheus.CounterOpts{
            Namespace: Namespace,
            Name:      "replset_last",
            Help:      "Last event times for replica set",
    }, []string{"event"})
    replSetTotalMembers = prometheus.NewGauge(prometheus.GaugeOpts{
            Namespace: Namespace,
            Subsystem: "replset",
            Name:      "members",
            Help:      "Number of members in replica set",
    })
    replSetTotalMembersWithData = prometheus.NewGauge(prometheus.GaugeOpts{
            Namespace: Namespace,
            Subsystem: "replset",
            Name:      "members_w_data",
            Help:      "Number of members in replica set with data",
    })
    replSetMyLagMs = prometheus.NewGauge(prometheus.GaugeOpts{
            Namespace: Namespace,
            Subsystem: "replset",
            Name:      "my_lag_ms",
            Help:      "Lag in milliseconds in reference to replica set Primary node",
    })
    replSetMaxNode2NodePingMs = prometheus.NewGauge(prometheus.GaugeOpts{
            Namespace: Namespace,
            Subsystem: "replset",
            Name:      "max_n2n_ping_ms",
            Help:      "Maximum ping in milliseconds to other replica set members",
    })
)

type ReplicaSetMemberStatus struct {
    Id			int64		`bson:"_id"`
    ConfigVersion	int64		`bson:"configVersion"`
    Health		int64		`bson:"health"`
    Name		string		`bson:"name"`
    State		int64		`bson:"state"`
    StateStr		string		`bson:"stateStr"`
    Uptime		int64		`bson:"uptime"`
    Optime		int64		`bson:"optime"`
    OptimeDate		time.Time	`bson:"optimeDate"`
    LastHeartbeat	time.Time	`bson:"lastHeartbeat"`
    LastHeartbeatRecv	time.Time	`bson:"lastHeartbeatRecv"`
    ElectionTime	int64		`bson:"electionTime"`
    ElectionDate	time.Time	`bson:"electionDate"`
    PingMs		float64		`bson:"pingMs"`
    SyncingTo		string		`bson:"syncingTo"`
    Self		bool		`bson:"self"`
}

type ReplicaSetStatus struct {
    Name	string				`bson:"set"`
    Date	time.Time			`bson:"date"`
    MyState	int				`bson:"myState"`
    Ok		int				`bson:"ok"`	
    Members	[]ReplicaSetMemberStatus	`bson:"members"`
}

type ReplicaSetStatusSummary struct {
    Members             float64
    MembersWithData     float64
    LagMs               float64
    MaxNode2NodePingMs  float64
    LastElection        float64
}

func GetReplSetStatusData(session *mgo.Session) (*ReplicaSetStatus, error) {
    replSetStatus := &ReplicaSetStatus{}

    err := session.DB("admin").Run(bson.D{{ "replSetGetStatus", 1 }}, &replSetStatus)

    return replSetStatus, err
}

func GetReplSetSelf(status *ReplicaSetStatus) *ReplicaSetMemberStatus {
    result := &ReplicaSetMemberStatus{}

    for _, member := range status.Members {
        if member.Self == true {
            result = &member
            break
        }
    }

    return result
}

func GetReplSetPrimary(status *ReplicaSetStatus) *ReplicaSetMemberStatus {
    result := &ReplicaSetMemberStatus{}

    for _, member := range status.Members {
        if member.State == 1 {
            result = &member
            break
        }
    }

    return result
}

func GetReplSetMemberByName(status *ReplicaSetStatus, name string) *ReplicaSetMemberStatus {
    result := &ReplicaSetMemberStatus{}

    for _, member := range status.Members {
        if member.Name == name {
            result = &member
            break
        }
    }

    return result
}

func GetReplSetSyncingTo(status *ReplicaSetStatus) *ReplicaSetMemberStatus {
    myInfo := GetReplSetSelf(status)
    if len(myInfo.SyncingTo) > 0 {
        return GetReplSetMemberByName(status, myInfo.SyncingTo)
    } else {
        return GetReplSetPrimary(status)
    }
}

func GetReplSetMemberCount(status *ReplicaSetStatus) (float64) {
    var result float64 = 0

    if status.Members != nil {
        result = float64(len(status.Members))
    }

    return result
}

func GetReplSetMembersWithDataCount(status *ReplicaSetStatus) (float64) {
    var membersWithDataCount int = 0

    if status.Members != nil {
        for _, member := range status.Members {
            if member.Health == 1 {
                if member.State == 1 || member.State == 2 {
                    membersWithDataCount = membersWithDataCount + 1
                }
            }
        }
    }

    return float64(membersWithDataCount)
}

func GetReplSetMaxNode2NodePingMs(status *ReplicaSetStatus) (float64) {
    var maxNodePingMs float64 = -1

    for _, member := range status.Members {
        if &member.PingMs != nil {
            if member.PingMs > maxNodePingMs {
                maxNodePingMs = member.PingMs
            }
        }
    }

    return maxNodePingMs
}

func GetReplSetLagMs(status *ReplicaSetStatus) (float64) {
    memberInfo := GetReplSetSelf(status)

    // short-circuit the check if you're the Primary
    if memberInfo.State == 1 {
        return 0
    }

    var result float64 = -1
    optimeNanoSelf := memberInfo.OptimeDate.UnixNano()
    replSetStatusPrimary := GetReplSetSyncingTo(status)
    if &replSetStatusPrimary.OptimeDate != nil {
        optimeNanoPrimary := replSetStatusPrimary.OptimeDate.UnixNano()
        result = float64(optimeNanoPrimary - optimeNanoSelf)/1000000
    }

    return result
}

func GetReplSetLastElectionUnixTime(status *ReplicaSetStatus) (float64) {
    replSetPrimary := GetReplSetPrimary(status)

    var result float64 = -1
    if &replSetPrimary.ElectionDate != nil {
        result = float64(replSetPrimary.ElectionDate.Unix())
    }

    return result
}

func(summary *ReplicaSetStatusSummary) Export(ch chan<- prometheus.Metric) {
    replSetTotalMembers.Set(summary.Members)
    replSetTotalMembersWithData.Set(summary.MembersWithData)
    replSetMyLagMs.Set(summary.LagMs)
    replSetMaxNode2NodePingMs.Set(summary.MaxNode2NodePingMs)
    replSetLastElection.WithLabelValues("election").Set(summary.LastElection)

    replSetTotalMembers.Collect(ch)
    replSetTotalMembersWithData.Collect(ch)
    replSetMyLagMs.Collect(ch)
    replSetMaxNode2NodePingMs.Collect(ch)
    replSetLastElection.Collect(ch)
}

func GetReplSetStatus(session *mgo.Session) *ReplicaSetStatusSummary {
    status, err := GetReplSetStatusData(session)
    if err != nil {
        glog.Error("Could not get replset status!")
    }

    summary := &ReplicaSetStatusSummary{}
    summary.Members = GetReplSetMemberCount(status)
    summary.MembersWithData = GetReplSetMembersWithDataCount(status)
    summary.LastElection = GetReplSetLastElectionUnixTime(status)
    summary.LagMs = GetReplSetLagMs(status)
    summary.MaxNode2NodePingMs = GetReplSetMaxNode2NodePingMs(status)

    return summary
}
