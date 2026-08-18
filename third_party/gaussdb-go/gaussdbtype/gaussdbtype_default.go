package gaussdbtype

import (
	"encoding/json"
	"encoding/xml"
	"net"
	"net/netip"
	"reflect"
	"sync"
	"time"
)

var (
	// defaultMap contains default mappings between GaussDB server types and Go type handling logic.
	defaultMap         *Map
	defaultMapInitOnce = sync.Once{}
)

func initDefaultMap() {
	defaultMap = &Map{
		oidToType:         make(map[uint32]*Type),
		nameToType:        make(map[string]*Type),
		reflectTypeToName: make(map[reflect.Type]string),
		oidToFormatCode:   make(map[uint32]int16),

		memoizedScanPlans:   make(map[uint32]map[reflect.Type][2]ScanPlan),
		memoizedEncodePlans: make(map[uint32]map[reflect.Type][2]EncodePlan),

		TryWrapEncodePlanFuncs: []TryWrapEncodePlanFunc{
			TryWrapDerefPointerEncodePlan,
			TryWrapBuiltinTypeEncodePlan,
			TryWrapFindUnderlyingTypeEncodePlan,
			TryWrapStructEncodePlan,
			TryWrapSliceEncodePlan,
			TryWrapMultiDimSliceEncodePlan,
			TryWrapArrayEncodePlan,
		},

		TryWrapScanPlanFuncs: []TryWrapScanPlanFunc{
			TryPointerPointerScanPlan,
			TryWrapBuiltinTypeScanPlan,
			TryFindUnderlyingTypeScanPlan,
			TryWrapStructScanPlan,
			TryWrapPtrSliceScanPlan,
			TryWrapPtrMultiDimSliceScanPlan,
			TryWrapPtrArrayScanPlan,
		},
	}

	// Base types
	defaultMap.RegisterType(&Type{Name: "aclitem", OID: ACLItemOID, Codec: &TextFormatOnlyCodec{TextCodec{}}})
	defaultMap.RegisterType(&Type{Name: "bit", OID: BitOID, Codec: BitsCodec{}})
	defaultMap.RegisterType(&Type{Name: "bool", OID: BoolOID, Codec: BoolCodec{}})
	defaultMap.RegisterType(&Type{Name: "box", OID: BoxOID, Codec: BoxCodec{}})
	defaultMap.RegisterType(&Type{Name: "bpchar", OID: BPCharOID, Codec: TextCodec{}})
	defaultMap.RegisterType(&Type{Name: "bytea", OID: ByteaOID, Codec: ByteaCodec{}})
	defaultMap.RegisterType(&Type{Name: "char", OID: QCharOID, Codec: QCharCodec{}})
	defaultMap.RegisterType(&Type{Name: "cid", OID: CIDOID, Codec: Uint32Codec{}})
	defaultMap.RegisterType(&Type{Name: "cidr", OID: CIDROID, Codec: InetCodec{}})
	defaultMap.RegisterType(&Type{Name: "circle", OID: CircleOID, Codec: CircleCodec{}})
	defaultMap.RegisterType(&Type{Name: "date", OID: DateOID, Codec: DateCodec{}})
	defaultMap.RegisterType(&Type{Name: "float4", OID: Float4OID, Codec: Float4Codec{}})
	defaultMap.RegisterType(&Type{Name: "float8", OID: Float8OID, Codec: Float8Codec{}})
	defaultMap.RegisterType(&Type{Name: "inet", OID: InetOID, Codec: InetCodec{}})
	defaultMap.RegisterType(&Type{Name: "int2", OID: Int2OID, Codec: Int2Codec{}})
	defaultMap.RegisterType(&Type{Name: "int4", OID: Int4OID, Codec: Int4Codec{}})
	defaultMap.RegisterType(&Type{Name: "int8", OID: Int8OID, Codec: Int8Codec{}})
	defaultMap.RegisterType(&Type{Name: "interval", OID: IntervalOID, Codec: IntervalCodec{}})
	defaultMap.RegisterType(&Type{Name: "json", OID: JSONOID, Codec: &JSONCodec{Marshal: json.Marshal, Unmarshal: json.Unmarshal}})
	defaultMap.RegisterType(&Type{Name: "jsonb", OID: JSONBOID, Codec: &JSONBCodec{Marshal: json.Marshal, Unmarshal: json.Unmarshal}})
	defaultMap.RegisterType(&Type{Name: "jsonpath", OID: JSONPathOID, Codec: &TextFormatOnlyCodec{TextCodec{}}})
	defaultMap.RegisterType(&Type{Name: "line", OID: LineOID, Codec: LineCodec{}})
	defaultMap.RegisterType(&Type{Name: "lseg", OID: LsegOID, Codec: LsegCodec{}})
	defaultMap.RegisterType(&Type{Name: "macaddr8", OID: Macaddr8OID, Codec: MacaddrCodec{}})
	defaultMap.RegisterType(&Type{Name: "macaddr", OID: MacaddrOID, Codec: MacaddrCodec{}})
	defaultMap.RegisterType(&Type{Name: "name", OID: NameOID, Codec: TextCodec{}})
	defaultMap.RegisterType(&Type{Name: "numeric", OID: NumericOID, Codec: NumericCodec{}})
	defaultMap.RegisterType(&Type{Name: "oid", OID: OIDOID, Codec: Uint32Codec{}})
	defaultMap.RegisterType(&Type{Name: "path", OID: PathOID, Codec: PathCodec{}})
	defaultMap.RegisterType(&Type{Name: "point", OID: PointOID, Codec: PointCodec{}})
	defaultMap.RegisterType(&Type{Name: "polygon", OID: PolygonOID, Codec: PolygonCodec{}})
	defaultMap.RegisterType(&Type{Name: "record", OID: RecordOID, Codec: RecordCodec{}})
	defaultMap.RegisterType(&Type{Name: "text", OID: TextOID, Codec: TextCodec{}})
	defaultMap.RegisterType(&Type{Name: "tid", OID: TIDOID, Codec: TIDCodec{}})
	defaultMap.RegisterType(&Type{Name: "time", OID: TimeOID, Codec: TimeCodec{}})
	defaultMap.RegisterType(&Type{Name: "timestamp", OID: TimestampOID, Codec: &TimestampCodec{}})
	defaultMap.RegisterType(&Type{Name: "timestamptz", OID: TimestamptzOID, Codec: &TimestamptzCodec{}})
	defaultMap.RegisterType(&Type{Name: "unknown", OID: UnknownOID, Codec: TextCodec{}})
	defaultMap.RegisterType(&Type{Name: "uuid", OID: UUIDOID, Codec: UUIDCodec{}})
	defaultMap.RegisterType(&Type{Name: "varbit", OID: VarbitOID, Codec: BitsCodec{}})
	defaultMap.RegisterType(&Type{Name: "varchar", OID: VarcharOID, Codec: TextCodec{}})
	defaultMap.RegisterType(&Type{Name: "xid", OID: XIDOID, Codec: Uint32Codec{}})
	defaultMap.RegisterType(&Type{Name: "xid8", OID: XID8OID, Codec: Uint64Codec{}})
	defaultMap.RegisterType(&Type{Name: "xml", OID: XMLOID, Codec: &XMLCodec{
		Marshal: xml.Marshal,
		// xml.Unmarshal does not support unmarshalling into *any. However, XMLCodec.DecodeValue calls Unmarshal with a
		// *any. Wrap xml.Marshal with a function that copies the data into a new byte slice in this case. Not implementing
		// directly in XMLCodec.DecodeValue to allow for the unlikely possibility that someone uses an alternative XML
		// unmarshaler that does support unmarshalling into *any.
		Unmarshal: func(data []byte, v any) error {
			if v, ok := v.(*any); ok {
				dstBuf := make([]byte, len(data))
				copy(dstBuf, data)
				*v = dstBuf
				return nil
			}
			return xml.Unmarshal(data, v)
		},
	}})

	// Range types
	defaultMap.RegisterType(&Type{Name: "daterange", OID: DaterangeOID, Codec: &RangeCodec{ElementType: defaultMap.oidToType[DateOID]}})
	defaultMap.RegisterType(&Type{Name: "int4range", OID: Int4rangeOID, Codec: &RangeCodec{ElementType: defaultMap.oidToType[Int4OID]}})
	defaultMap.RegisterType(&Type{Name: "int8range", OID: Int8rangeOID, Codec: &RangeCodec{ElementType: defaultMap.oidToType[Int8OID]}})
	defaultMap.RegisterType(&Type{Name: "numrange", OID: NumrangeOID, Codec: &RangeCodec{ElementType: defaultMap.oidToType[NumericOID]}})
	defaultMap.RegisterType(&Type{Name: "tsrange", OID: TsrangeOID, Codec: &RangeCodec{ElementType: defaultMap.oidToType[TimestampOID]}})
	defaultMap.RegisterType(&Type{Name: "tstzrange", OID: TstzrangeOID, Codec: &RangeCodec{ElementType: defaultMap.oidToType[TimestamptzOID]}})

	// Multirange types
	defaultMap.RegisterType(&Type{Name: "datemultirange", OID: DatemultirangeOID, Codec: &MultirangeCodec{ElementType: defaultMap.oidToType[DaterangeOID]}})
	defaultMap.RegisterType(&Type{Name: "int4multirange", OID: Int4multirangeOID, Codec: &MultirangeCodec{ElementType: defaultMap.oidToType[Int4rangeOID]}})
	defaultMap.RegisterType(&Type{Name: "int8multirange", OID: Int8multirangeOID, Codec: &MultirangeCodec{ElementType: defaultMap.oidToType[Int8rangeOID]}})
	defaultMap.RegisterType(&Type{Name: "nummultirange", OID: NummultirangeOID, Codec: &MultirangeCodec{ElementType: defaultMap.oidToType[NumrangeOID]}})
	defaultMap.RegisterType(&Type{Name: "tsmultirange", OID: TsmultirangeOID, Codec: &MultirangeCodec{ElementType: defaultMap.oidToType[TsrangeOID]}})
	defaultMap.RegisterType(&Type{Name: "tstzmultirange", OID: TstzmultirangeOID, Codec: &MultirangeCodec{ElementType: defaultMap.oidToType[TstzrangeOID]}})

	// Array types
	defaultMap.RegisterType(&Type{Name: "_aclitem", OID: ACLItemArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[ACLItemOID]}})
	defaultMap.RegisterType(&Type{Name: "_bit", OID: BitArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[BitOID]}})
	defaultMap.RegisterType(&Type{Name: "_bool", OID: BoolArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[BoolOID]}})
	defaultMap.RegisterType(&Type{Name: "_box", OID: BoxArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[BoxOID]}})
	defaultMap.RegisterType(&Type{Name: "_bpchar", OID: BPCharArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[BPCharOID]}})
	defaultMap.RegisterType(&Type{Name: "_bytea", OID: ByteaArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[ByteaOID]}})
	defaultMap.RegisterType(&Type{Name: "_char", OID: QCharArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[QCharOID]}})
	defaultMap.RegisterType(&Type{Name: "_cid", OID: CIDArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[CIDOID]}})
	defaultMap.RegisterType(&Type{Name: "_cidr", OID: CIDRArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[CIDROID]}})
	defaultMap.RegisterType(&Type{Name: "_circle", OID: CircleArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[CircleOID]}})
	defaultMap.RegisterType(&Type{Name: "_date", OID: DateArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[DateOID]}})
	defaultMap.RegisterType(&Type{Name: "_daterange", OID: DaterangeArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[DaterangeOID]}})
	defaultMap.RegisterType(&Type{Name: "_float4", OID: Float4ArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[Float4OID]}})
	defaultMap.RegisterType(&Type{Name: "_float8", OID: Float8ArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[Float8OID]}})
	defaultMap.RegisterType(&Type{Name: "_inet", OID: InetArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[InetOID]}})
	defaultMap.RegisterType(&Type{Name: "_int2", OID: Int2ArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[Int2OID]}})
	defaultMap.RegisterType(&Type{Name: "_int4", OID: Int4ArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[Int4OID]}})
	defaultMap.RegisterType(&Type{Name: "_int4range", OID: Int4rangeArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[Int4rangeOID]}})
	defaultMap.RegisterType(&Type{Name: "_int8", OID: Int8ArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[Int8OID]}})
	defaultMap.RegisterType(&Type{Name: "_int8range", OID: Int8rangeArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[Int8rangeOID]}})
	defaultMap.RegisterType(&Type{Name: "_interval", OID: IntervalArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[IntervalOID]}})
	defaultMap.RegisterType(&Type{Name: "_json", OID: JSONArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[JSONOID]}})
	defaultMap.RegisterType(&Type{Name: "_jsonb", OID: JSONBArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[JSONBOID]}})
	defaultMap.RegisterType(&Type{Name: "_jsonpath", OID: JSONPathArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[JSONPathOID]}})
	defaultMap.RegisterType(&Type{Name: "_line", OID: LineArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[LineOID]}})
	defaultMap.RegisterType(&Type{Name: "_lseg", OID: LsegArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[LsegOID]}})
	defaultMap.RegisterType(&Type{Name: "_macaddr", OID: MacaddrArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[MacaddrOID]}})
	defaultMap.RegisterType(&Type{Name: "_name", OID: NameArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[NameOID]}})
	defaultMap.RegisterType(&Type{Name: "_numeric", OID: NumericArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[NumericOID]}})
	defaultMap.RegisterType(&Type{Name: "_numrange", OID: NumrangeArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[NumrangeOID]}})
	defaultMap.RegisterType(&Type{Name: "_oid", OID: OIDArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[OIDOID]}})
	defaultMap.RegisterType(&Type{Name: "_path", OID: PathArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[PathOID]}})
	defaultMap.RegisterType(&Type{Name: "_point", OID: PointArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[PointOID]}})
	defaultMap.RegisterType(&Type{Name: "_polygon", OID: PolygonArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[PolygonOID]}})
	defaultMap.RegisterType(&Type{Name: "_record", OID: RecordArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[RecordOID]}})
	defaultMap.RegisterType(&Type{Name: "_text", OID: TextArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[TextOID]}})
	defaultMap.RegisterType(&Type{Name: "_tid", OID: TIDArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[TIDOID]}})
	defaultMap.RegisterType(&Type{Name: "_time", OID: TimeArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[TimeOID]}})
	defaultMap.RegisterType(&Type{Name: "_timestamp", OID: TimestampArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[TimestampOID]}})
	defaultMap.RegisterType(&Type{Name: "_timestamptz", OID: TimestamptzArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[TimestamptzOID]}})
	defaultMap.RegisterType(&Type{Name: "_tsrange", OID: TsrangeArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[TsrangeOID]}})
	defaultMap.RegisterType(&Type{Name: "_tstzrange", OID: TstzrangeArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[TstzrangeOID]}})
	defaultMap.RegisterType(&Type{Name: "_uuid", OID: UUIDArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[UUIDOID]}})
	defaultMap.RegisterType(&Type{Name: "_varbit", OID: VarbitArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[VarbitOID]}})
	defaultMap.RegisterType(&Type{Name: "_varchar", OID: VarcharArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[VarcharOID]}})
	defaultMap.RegisterType(&Type{Name: "_xid", OID: XIDArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[XIDOID]}})
	defaultMap.RegisterType(&Type{Name: "_xid8", OID: XID8ArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[XID8OID]}})
	defaultMap.RegisterType(&Type{Name: "_xml", OID: XMLArrayOID, Codec: &ArrayCodec{ElementType: defaultMap.oidToType[XMLOID]}})

	// Integer types that directly map to a GaussDB type
	registerDefaultGaussdbTypeVariants[int16](defaultMap, "int2")
	registerDefaultGaussdbTypeVariants[int32](defaultMap, "int4")
	registerDefaultGaussdbTypeVariants[int64](defaultMap, "int8")

	// Integer types that do not have a direct match to a GaussDB type
	registerDefaultGaussdbTypeVariants[int8](defaultMap, "int8")
	registerDefaultGaussdbTypeVariants[int](defaultMap, "int8")
	registerDefaultGaussdbTypeVariants[uint8](defaultMap, "int8")
	registerDefaultGaussdbTypeVariants[uint16](defaultMap, "int8")
	registerDefaultGaussdbTypeVariants[uint32](defaultMap, "int8")
	registerDefaultGaussdbTypeVariants[uint64](defaultMap, "numeric")
	registerDefaultGaussdbTypeVariants[uint](defaultMap, "numeric")

	registerDefaultGaussdbTypeVariants[float32](defaultMap, "float4")
	registerDefaultGaussdbTypeVariants[float64](defaultMap, "float8")

	registerDefaultGaussdbTypeVariants[bool](defaultMap, "bool")
	registerDefaultGaussdbTypeVariants[time.Time](defaultMap, "timestamptz")
	registerDefaultGaussdbTypeVariants[time.Duration](defaultMap, "interval")
	registerDefaultGaussdbTypeVariants[string](defaultMap, "text")
	registerDefaultGaussdbTypeVariants[json.RawMessage](defaultMap, "json")
	registerDefaultGaussdbTypeVariants[[]byte](defaultMap, "bytea")

	registerDefaultGaussdbTypeVariants[net.IP](defaultMap, "inet")
	registerDefaultGaussdbTypeVariants[net.IPNet](defaultMap, "cidr")
	registerDefaultGaussdbTypeVariants[netip.Addr](defaultMap, "inet")
	registerDefaultGaussdbTypeVariants[netip.Prefix](defaultMap, "cidr")

	// gaussdbtype provided structs
	registerDefaultGaussdbTypeVariants[Bits](defaultMap, "varbit")
	registerDefaultGaussdbTypeVariants[Bool](defaultMap, "bool")
	registerDefaultGaussdbTypeVariants[Box](defaultMap, "box")
	registerDefaultGaussdbTypeVariants[Circle](defaultMap, "circle")
	registerDefaultGaussdbTypeVariants[Date](defaultMap, "date")
	registerDefaultGaussdbTypeVariants[Range[Date]](defaultMap, "daterange")
	registerDefaultGaussdbTypeVariants[Multirange[Range[Date]]](defaultMap, "datemultirange")
	registerDefaultGaussdbTypeVariants[Float4](defaultMap, "float4")
	registerDefaultGaussdbTypeVariants[Float8](defaultMap, "float8")
	registerDefaultGaussdbTypeVariants[Range[Float8]](defaultMap, "numrange")                  // There is no GaussDB builtin float8range so map it to numrange.
	registerDefaultGaussdbTypeVariants[Multirange[Range[Float8]]](defaultMap, "nummultirange") // There is no GaussDB builtin float8multirange so map it to nummultirange.
	registerDefaultGaussdbTypeVariants[Int2](defaultMap, "int2")
	registerDefaultGaussdbTypeVariants[Int4](defaultMap, "int4")
	registerDefaultGaussdbTypeVariants[Range[Int4]](defaultMap, "int4range")
	registerDefaultGaussdbTypeVariants[Multirange[Range[Int4]]](defaultMap, "int4multirange")
	registerDefaultGaussdbTypeVariants[Int8](defaultMap, "int8")
	registerDefaultGaussdbTypeVariants[Range[Int8]](defaultMap, "int8range")
	registerDefaultGaussdbTypeVariants[Multirange[Range[Int8]]](defaultMap, "int8multirange")
	registerDefaultGaussdbTypeVariants[Interval](defaultMap, "interval")
	registerDefaultGaussdbTypeVariants[Line](defaultMap, "line")
	registerDefaultGaussdbTypeVariants[Lseg](defaultMap, "lseg")
	registerDefaultGaussdbTypeVariants[Numeric](defaultMap, "numeric")
	registerDefaultGaussdbTypeVariants[Range[Numeric]](defaultMap, "numrange")
	registerDefaultGaussdbTypeVariants[Multirange[Range[Numeric]]](defaultMap, "nummultirange")
	registerDefaultGaussdbTypeVariants[Path](defaultMap, "path")
	registerDefaultGaussdbTypeVariants[Point](defaultMap, "point")
	registerDefaultGaussdbTypeVariants[Polygon](defaultMap, "polygon")
	registerDefaultGaussdbTypeVariants[TID](defaultMap, "tid")
	registerDefaultGaussdbTypeVariants[Text](defaultMap, "text")
	registerDefaultGaussdbTypeVariants[Time](defaultMap, "time")
	registerDefaultGaussdbTypeVariants[Timestamp](defaultMap, "timestamp")
	registerDefaultGaussdbTypeVariants[Timestamptz](defaultMap, "timestamptz")
	registerDefaultGaussdbTypeVariants[Range[Timestamp]](defaultMap, "tsrange")
	registerDefaultGaussdbTypeVariants[Multirange[Range[Timestamp]]](defaultMap, "tsmultirange")
	registerDefaultGaussdbTypeVariants[Range[Timestamptz]](defaultMap, "tstzrange")
	registerDefaultGaussdbTypeVariants[Multirange[Range[Timestamptz]]](defaultMap, "tstzmultirange")
	registerDefaultGaussdbTypeVariants[UUID](defaultMap, "uuid")

	defaultMap.buildReflectTypeToType()
}
