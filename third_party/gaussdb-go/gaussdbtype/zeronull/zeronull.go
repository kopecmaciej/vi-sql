package zeronull

import (
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
)

// Register registers the zeronull types so they can be used in query exec modes that do not know the server OIDs.
func Register(m *gaussdbtype.Map) {
	m.RegisterDefaultGaussdbType(Float8(0), "float8")
	m.RegisterDefaultGaussdbType(Int2(0), "int2")
	m.RegisterDefaultGaussdbType(Int4(0), "int4")
	m.RegisterDefaultGaussdbType(Int8(0), "int8")
	m.RegisterDefaultGaussdbType(Text(""), "text")
	m.RegisterDefaultGaussdbType(Timestamp{}, "timestamp")
	m.RegisterDefaultGaussdbType(Timestamptz{}, "timestamptz")
	m.RegisterDefaultGaussdbType(UUID{}, "uuid")
}
