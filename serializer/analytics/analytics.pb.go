// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.0
// 	protoc        v3.14.0
// source: analytics.proto

package analytics

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type AnalyticsRecord struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Host          string                 `protobuf:"bytes,1,opt,name=Host,proto3" json:"Host,omitempty"`
	Method        string                 `protobuf:"bytes,2,opt,name=Method,proto3" json:"Method,omitempty"`
	Path          string                 `protobuf:"bytes,3,opt,name=Path,proto3" json:"Path,omitempty"`
	RawPath       string                 `protobuf:"bytes,4,opt,name=RawPath,proto3" json:"RawPath,omitempty"`
	ContentLength int64                  `protobuf:"varint,5,opt,name=ContentLength,proto3" json:"ContentLength,omitempty"`
	UserAgent     string                 `protobuf:"bytes,6,opt,name=UserAgent,proto3" json:"UserAgent,omitempty"`
	Day           int32                  `protobuf:"varint,7,opt,name=Day,proto3" json:"Day,omitempty"`
	Month         int32                  `protobuf:"varint,8,opt,name=Month,proto3" json:"Month,omitempty"`
	Year          int32                  `protobuf:"varint,9,opt,name=Year,proto3" json:"Year,omitempty"`
	Hour          int32                  `protobuf:"varint,10,opt,name=Hour,proto3" json:"Hour,omitempty"`
	ResponseCode  int32                  `protobuf:"varint,11,opt,name=ResponseCode,proto3" json:"ResponseCode,omitempty"`
	APIKey        string                 `protobuf:"bytes,12,opt,name=APIKey,proto3" json:"APIKey,omitempty"`
	TimeStamp     *timestamppb.Timestamp `protobuf:"bytes,13,opt,name=TimeStamp,proto3" json:"TimeStamp,omitempty"`
	APIVersion    string                 `protobuf:"bytes,14,opt,name=APIVersion,proto3" json:"APIVersion,omitempty"`
	APIName       string                 `protobuf:"bytes,15,opt,name=APIName,proto3" json:"APIName,omitempty"`
	APIID         string                 `protobuf:"bytes,16,opt,name=APIID,proto3" json:"APIID,omitempty"`
	OrgID         string                 `protobuf:"bytes,17,opt,name=OrgID,proto3" json:"OrgID,omitempty"`
	RequestTime   int64                  `protobuf:"varint,18,opt,name=RequestTime,proto3" json:"RequestTime,omitempty"`
	Latency       *Latency               `protobuf:"bytes,19,opt,name=Latency,proto3" json:"Latency,omitempty"`
	RawRequest    string                 `protobuf:"bytes,20,opt,name=RawRequest,proto3" json:"RawRequest,omitempty"`
	RawResponse   string                 `protobuf:"bytes,21,opt,name=RawResponse,proto3" json:"RawResponse,omitempty"`
	IPAddress     string                 `protobuf:"bytes,22,opt,name=IPAddress,proto3" json:"IPAddress,omitempty"`
	Geo           *GeoData               `protobuf:"bytes,23,opt,name=Geo,proto3" json:"Geo,omitempty"`
	Network       *NetworkStats          `protobuf:"bytes,24,opt,name=Network,proto3" json:"Network,omitempty"`
	Tags          []string               `protobuf:"bytes,25,rep,name=Tags,proto3" json:"Tags,omitempty"`
	Alias         string                 `protobuf:"bytes,26,opt,name=Alias,proto3" json:"Alias,omitempty"`
	TrackPath     bool                   `protobuf:"varint,27,opt,name=TrackPath,proto3" json:"TrackPath,omitempty"`
	ExpireAt      *timestamppb.Timestamp `protobuf:"bytes,28,opt,name=ExpireAt,proto3" json:"ExpireAt,omitempty"`
	OauthID       string                 `protobuf:"bytes,29,opt,name=OauthID,proto3" json:"OauthID,omitempty"`
	TimeZone      string                 `protobuf:"bytes,30,opt,name=TimeZone,proto3" json:"TimeZone,omitempty"`
}

func (x *AnalyticsRecord) Reset() {
	*x = AnalyticsRecord{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analytics_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AnalyticsRecord) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AnalyticsRecord) ProtoMessage() {}

func (x *AnalyticsRecord) ProtoReflect() protoreflect.Message {
	mi := &file_analytics_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AnalyticsRecord.ProtoReflect.Descriptor instead.
func (*AnalyticsRecord) Descriptor() ([]byte, []int) {
	return file_analytics_proto_rawDescGZIP(), []int{0}
}

func (x *AnalyticsRecord) GetHost() string {
	if x != nil {
		return x.Host
	}
	return ""
}

func (x *AnalyticsRecord) GetMethod() string {
	if x != nil {
		return x.Method
	}
	return ""
}

func (x *AnalyticsRecord) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (x *AnalyticsRecord) GetRawPath() string {
	if x != nil {
		return x.RawPath
	}
	return ""
}

func (x *AnalyticsRecord) GetContentLength() int64 {
	if x != nil {
		return x.ContentLength
	}
	return 0
}

func (x *AnalyticsRecord) GetUserAgent() string {
	if x != nil {
		return x.UserAgent
	}
	return ""
}

func (x *AnalyticsRecord) GetDay() int32 {
	if x != nil {
		return x.Day
	}
	return 0
}

func (x *AnalyticsRecord) GetMonth() int32 {
	if x != nil {
		return x.Month
	}
	return 0
}

func (x *AnalyticsRecord) GetYear() int32 {
	if x != nil {
		return x.Year
	}
	return 0
}

func (x *AnalyticsRecord) GetHour() int32 {
	if x != nil {
		return x.Hour
	}
	return 0
}

func (x *AnalyticsRecord) GetResponseCode() int32 {
	if x != nil {
		return x.ResponseCode
	}
	return 0
}

func (x *AnalyticsRecord) GetAPIKey() string {
	if x != nil {
		return x.APIKey
	}
	return ""
}

func (x *AnalyticsRecord) GetTimeStamp() *timestamppb.Timestamp {
	if x != nil {
		return x.TimeStamp
	}
	return nil
}

func (x *AnalyticsRecord) GetAPIVersion() string {
	if x != nil {
		return x.APIVersion
	}
	return ""
}

func (x *AnalyticsRecord) GetAPIName() string {
	if x != nil {
		return x.APIName
	}
	return ""
}

func (x *AnalyticsRecord) GetAPIID() string {
	if x != nil {
		return x.APIID
	}
	return ""
}

func (x *AnalyticsRecord) GetOrgID() string {
	if x != nil {
		return x.OrgID
	}
	return ""
}

func (x *AnalyticsRecord) GetRequestTime() int64 {
	if x != nil {
		return x.RequestTime
	}
	return 0
}

func (x *AnalyticsRecord) GetLatency() *Latency {
	if x != nil {
		return x.Latency
	}
	return nil
}

func (x *AnalyticsRecord) GetRawRequest() string {
	if x != nil {
		return x.RawRequest
	}
	return ""
}

func (x *AnalyticsRecord) GetRawResponse() string {
	if x != nil {
		return x.RawResponse
	}
	return ""
}

func (x *AnalyticsRecord) GetIPAddress() string {
	if x != nil {
		return x.IPAddress
	}
	return ""
}

func (x *AnalyticsRecord) GetGeo() *GeoData {
	if x != nil {
		return x.Geo
	}
	return nil
}

func (x *AnalyticsRecord) GetNetwork() *NetworkStats {
	if x != nil {
		return x.Network
	}
	return nil
}

func (x *AnalyticsRecord) GetTags() []string {
	if x != nil {
		return x.Tags
	}
	return nil
}

func (x *AnalyticsRecord) GetAlias() string {
	if x != nil {
		return x.Alias
	}
	return ""
}

func (x *AnalyticsRecord) GetTrackPath() bool {
	if x != nil {
		return x.TrackPath
	}
	return false
}

func (x *AnalyticsRecord) GetExpireAt() *timestamppb.Timestamp {
	if x != nil {
		return x.ExpireAt
	}
	return nil
}

func (x *AnalyticsRecord) GetOauthID() string {
	if x != nil {
		return x.OauthID
	}
	return ""
}

func (x *AnalyticsRecord) GetTimeZone() string {
	if x != nil {
		return x.TimeZone
	}
	return ""
}

type Latency struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Total    int64 `protobuf:"varint,1,opt,name=Total,proto3" json:"Total,omitempty"`
	Upstream int64 `protobuf:"varint,2,opt,name=Upstream,proto3" json:"Upstream,omitempty"`
}

func (x *Latency) Reset() {
	*x = Latency{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analytics_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Latency) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Latency) ProtoMessage() {}

func (x *Latency) ProtoReflect() protoreflect.Message {
	mi := &file_analytics_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Latency.ProtoReflect.Descriptor instead.
func (*Latency) Descriptor() ([]byte, []int) {
	return file_analytics_proto_rawDescGZIP(), []int{1}
}

func (x *Latency) GetTotal() int64 {
	if x != nil {
		return x.Total
	}
	return 0
}

func (x *Latency) GetUpstream() int64 {
	if x != nil {
		return x.Upstream
	}
	return 0
}

type Country struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ISOCode string `protobuf:"bytes,1,opt,name=ISOCode,proto3" json:"ISOCode,omitempty"`
}

func (x *Country) Reset() {
	*x = Country{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analytics_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Country) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Country) ProtoMessage() {}

func (x *Country) ProtoReflect() protoreflect.Message {
	mi := &file_analytics_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Country.ProtoReflect.Descriptor instead.
func (*Country) Descriptor() ([]byte, []int) {
	return file_analytics_proto_rawDescGZIP(), []int{2}
}

func (x *Country) GetISOCode() string {
	if x != nil {
		return x.ISOCode
	}
	return ""
}

type City struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Names map[string]string `protobuf:"bytes,1,rep,name=Names,proto3" json:"Names,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *City) Reset() {
	*x = City{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analytics_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *City) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*City) ProtoMessage() {}

func (x *City) ProtoReflect() protoreflect.Message {
	mi := &file_analytics_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use City.ProtoReflect.Descriptor instead.
func (*City) Descriptor() ([]byte, []int) {
	return file_analytics_proto_rawDescGZIP(), []int{3}
}

func (x *City) GetNames() map[string]string {
	if x != nil {
		return x.Names
	}
	return nil
}

type Location struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Latitude  float64 `protobuf:"fixed64,1,opt,name=Latitude,proto3" json:"Latitude,omitempty"`
	Longitude float64 `protobuf:"fixed64,2,opt,name=Longitude,proto3" json:"Longitude,omitempty"`
	TimeZone  string  `protobuf:"bytes,3,opt,name=TimeZone,proto3" json:"TimeZone,omitempty"`
}

func (x *Location) Reset() {
	*x = Location{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analytics_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Location) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Location) ProtoMessage() {}

func (x *Location) ProtoReflect() protoreflect.Message {
	mi := &file_analytics_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Location.ProtoReflect.Descriptor instead.
func (*Location) Descriptor() ([]byte, []int) {
	return file_analytics_proto_rawDescGZIP(), []int{4}
}

func (x *Location) GetLatitude() float64 {
	if x != nil {
		return x.Latitude
	}
	return 0
}

func (x *Location) GetLongitude() float64 {
	if x != nil {
		return x.Longitude
	}
	return 0
}

func (x *Location) GetTimeZone() string {
	if x != nil {
		return x.TimeZone
	}
	return ""
}

type GeoData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Country  *Country  `protobuf:"bytes,1,opt,name=Country,proto3" json:"Country,omitempty"`
	City     *City     `protobuf:"bytes,2,opt,name=City,proto3" json:"City,omitempty"`
	Location *Location `protobuf:"bytes,3,opt,name=Location,proto3" json:"Location,omitempty"`
}

func (x *GeoData) Reset() {
	*x = GeoData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analytics_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GeoData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GeoData) ProtoMessage() {}

func (x *GeoData) ProtoReflect() protoreflect.Message {
	mi := &file_analytics_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GeoData.ProtoReflect.Descriptor instead.
func (*GeoData) Descriptor() ([]byte, []int) {
	return file_analytics_proto_rawDescGZIP(), []int{5}
}

func (x *GeoData) GetCountry() *Country {
	if x != nil {
		return x.Country
	}
	return nil
}

func (x *GeoData) GetCity() *City {
	if x != nil {
		return x.City
	}
	return nil
}

func (x *GeoData) GetLocation() *Location {
	if x != nil {
		return x.Location
	}
	return nil
}

type NetworkStats struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	OpenConnections   int64 `protobuf:"varint,1,opt,name=OpenConnections,proto3" json:"OpenConnections,omitempty"`
	ClosedConnections int64 `protobuf:"varint,2,opt,name=ClosedConnections,proto3" json:"ClosedConnections,omitempty"`
	BytesIn           int64 `protobuf:"varint,3,opt,name=BytesIn,proto3" json:"BytesIn,omitempty"`
	BytesOut          int64 `protobuf:"varint,4,opt,name=BytesOut,proto3" json:"BytesOut,omitempty"`
}

func (x *NetworkStats) Reset() {
	*x = NetworkStats{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analytics_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NetworkStats) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NetworkStats) ProtoMessage() {}

func (x *NetworkStats) ProtoReflect() protoreflect.Message {
	mi := &file_analytics_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NetworkStats.ProtoReflect.Descriptor instead.
func (*NetworkStats) Descriptor() ([]byte, []int) {
	return file_analytics_proto_rawDescGZIP(), []int{6}
}

func (x *NetworkStats) GetOpenConnections() int64 {
	if x != nil {
		return x.OpenConnections
	}
	return 0
}

func (x *NetworkStats) GetClosedConnections() int64 {
	if x != nil {
		return x.ClosedConnections
	}
	return 0
}

func (x *NetworkStats) GetBytesIn() int64 {
	if x != nil {
		return x.BytesIn
	}
	return 0
}

func (x *NetworkStats) GetBytesOut() int64 {
	if x != nil {
		return x.BytesOut
	}
	return 0
}

var File_analytics_proto protoreflect.FileDescriptor

var file_analytics_proto_rawDesc = []byte{
	0x0a, 0x0f, 0x61, 0x6e, 0x61, 0x6c, 0x79, 0x74, 0x69, 0x63, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x0b, 0x6e, 0x6f, 0x72, 0x6d, 0x61, 0x6c, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1f,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f,
	0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22,
	0xa0, 0x07, 0x0a, 0x0f, 0x41, 0x6e, 0x61, 0x6c, 0x79, 0x74, 0x69, 0x63, 0x73, 0x52, 0x65, 0x63,
	0x6f, 0x72, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x48, 0x6f, 0x73, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x48, 0x6f, 0x73, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x4d, 0x65, 0x74, 0x68, 0x6f,
	0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x12,
	0x12, 0x0a, 0x04, 0x50, 0x61, 0x74, 0x68, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x50,
	0x61, 0x74, 0x68, 0x12, 0x18, 0x0a, 0x07, 0x52, 0x61, 0x77, 0x50, 0x61, 0x74, 0x68, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x52, 0x61, 0x77, 0x50, 0x61, 0x74, 0x68, 0x12, 0x24, 0x0a,
	0x0d, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x4c, 0x65, 0x6e, 0x67, 0x74, 0x68, 0x18, 0x05,
	0x20, 0x01, 0x28, 0x03, 0x52, 0x0d, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x4c, 0x65, 0x6e,
	0x67, 0x74, 0x68, 0x12, 0x1c, 0x0a, 0x09, 0x55, 0x73, 0x65, 0x72, 0x41, 0x67, 0x65, 0x6e, 0x74,
	0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x55, 0x73, 0x65, 0x72, 0x41, 0x67, 0x65, 0x6e,
	0x74, 0x12, 0x10, 0x0a, 0x03, 0x44, 0x61, 0x79, 0x18, 0x07, 0x20, 0x01, 0x28, 0x05, 0x52, 0x03,
	0x44, 0x61, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x4d, 0x6f, 0x6e, 0x74, 0x68, 0x18, 0x08, 0x20, 0x01,
	0x28, 0x05, 0x52, 0x05, 0x4d, 0x6f, 0x6e, 0x74, 0x68, 0x12, 0x12, 0x0a, 0x04, 0x59, 0x65, 0x61,
	0x72, 0x18, 0x09, 0x20, 0x01, 0x28, 0x05, 0x52, 0x04, 0x59, 0x65, 0x61, 0x72, 0x12, 0x12, 0x0a,
	0x04, 0x48, 0x6f, 0x75, 0x72, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x05, 0x52, 0x04, 0x48, 0x6f, 0x75,
	0x72, 0x12, 0x22, 0x0a, 0x0c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x43, 0x6f, 0x64,
	0x65, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x41, 0x50, 0x49, 0x4b, 0x65, 0x79, 0x18,
	0x0c, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x41, 0x50, 0x49, 0x4b, 0x65, 0x79, 0x12, 0x38, 0x0a,
	0x09, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x54, 0x69,
	0x6d, 0x65, 0x53, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x1e, 0x0a, 0x0a, 0x41, 0x50, 0x49, 0x56, 0x65,
	0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x41, 0x50, 0x49,
	0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x41, 0x50, 0x49, 0x4e, 0x61,
	0x6d, 0x65, 0x18, 0x0f, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x41, 0x50, 0x49, 0x4e, 0x61, 0x6d,
	0x65, 0x12, 0x14, 0x0a, 0x05, 0x41, 0x50, 0x49, 0x49, 0x44, 0x18, 0x10, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x05, 0x41, 0x50, 0x49, 0x49, 0x44, 0x12, 0x14, 0x0a, 0x05, 0x4f, 0x72, 0x67, 0x49, 0x44,
	0x18, 0x11, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x4f, 0x72, 0x67, 0x49, 0x44, 0x12, 0x20, 0x0a,
	0x0b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x18, 0x12, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x0b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x12,
	0x2e, 0x0a, 0x07, 0x4c, 0x61, 0x74, 0x65, 0x6e, 0x63, 0x79, 0x18, 0x13, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x14, 0x2e, 0x6e, 0x6f, 0x72, 0x6d, 0x61, 0x6c, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x4c,
	0x61, 0x74, 0x65, 0x6e, 0x63, 0x79, 0x52, 0x07, 0x4c, 0x61, 0x74, 0x65, 0x6e, 0x63, 0x79, 0x12,
	0x1e, 0x0a, 0x0a, 0x52, 0x61, 0x77, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x18, 0x14, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0a, 0x52, 0x61, 0x77, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x20, 0x0a, 0x0b, 0x52, 0x61, 0x77, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x18, 0x15,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x52, 0x61, 0x77, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x1c, 0x0a, 0x09, 0x49, 0x50, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x16,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x49, 0x50, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12,
	0x26, 0x0a, 0x03, 0x47, 0x65, 0x6f, 0x18, 0x17, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x6e,
	0x6f, 0x72, 0x6d, 0x61, 0x6c, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x47, 0x65, 0x6f, 0x44, 0x61,
	0x74, 0x61, 0x52, 0x03, 0x47, 0x65, 0x6f, 0x12, 0x33, 0x0a, 0x07, 0x4e, 0x65, 0x74, 0x77, 0x6f,
	0x72, 0x6b, 0x18, 0x18, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x6e, 0x6f, 0x72, 0x6d, 0x61,
	0x6c, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x53, 0x74,
	0x61, 0x74, 0x73, 0x52, 0x07, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x12, 0x12, 0x0a, 0x04,
	0x54, 0x61, 0x67, 0x73, 0x18, 0x19, 0x20, 0x03, 0x28, 0x09, 0x52, 0x04, 0x54, 0x61, 0x67, 0x73,
	0x12, 0x14, 0x0a, 0x05, 0x41, 0x6c, 0x69, 0x61, 0x73, 0x18, 0x1a, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x05, 0x41, 0x6c, 0x69, 0x61, 0x73, 0x12, 0x1c, 0x0a, 0x09, 0x54, 0x72, 0x61, 0x63, 0x6b, 0x50,
	0x61, 0x74, 0x68, 0x18, 0x1b, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x54, 0x72, 0x61, 0x63, 0x6b,
	0x50, 0x61, 0x74, 0x68, 0x12, 0x36, 0x0a, 0x08, 0x45, 0x78, 0x70, 0x69, 0x72, 0x65, 0x41, 0x74,
	0x18, 0x1c, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61,
	0x6d, 0x70, 0x52, 0x08, 0x45, 0x78, 0x70, 0x69, 0x72, 0x65, 0x41, 0x74, 0x12, 0x18, 0x0a, 0x07,
	0x4f, 0x61, 0x75, 0x74, 0x68, 0x49, 0x44, 0x18, 0x1d, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x4f,
	0x61, 0x75, 0x74, 0x68, 0x49, 0x44, 0x12, 0x1a, 0x0a, 0x08, 0x54, 0x69, 0x6d, 0x65, 0x5a, 0x6f,
	0x6e, 0x65, 0x18, 0x1e, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x54, 0x69, 0x6d, 0x65, 0x5a, 0x6f,
	0x6e, 0x65, 0x22, 0x3b, 0x0a, 0x07, 0x4c, 0x61, 0x74, 0x65, 0x6e, 0x63, 0x79, 0x12, 0x14, 0x0a,
	0x05, 0x54, 0x6f, 0x74, 0x61, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x54, 0x6f,
	0x74, 0x61, 0x6c, 0x12, 0x1a, 0x0a, 0x08, 0x55, 0x70, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x55, 0x70, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x22,
	0x23, 0x0a, 0x07, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x18, 0x0a, 0x07, 0x49, 0x53,
	0x4f, 0x43, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x49, 0x53, 0x4f,
	0x43, 0x6f, 0x64, 0x65, 0x22, 0x74, 0x0a, 0x04, 0x43, 0x69, 0x74, 0x79, 0x12, 0x32, 0x0a, 0x05,
	0x4e, 0x61, 0x6d, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x6e, 0x6f,
	0x72, 0x6d, 0x61, 0x6c, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x43, 0x69, 0x74, 0x79, 0x2e, 0x4e,
	0x61, 0x6d, 0x65, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x05, 0x4e, 0x61, 0x6d, 0x65, 0x73,
	0x1a, 0x38, 0x0a, 0x0a, 0x4e, 0x61, 0x6d, 0x65, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10,
	0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79,
	0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x60, 0x0a, 0x08, 0x4c, 0x6f,
	0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x1a, 0x0a, 0x08, 0x4c, 0x61, 0x74, 0x69, 0x74, 0x75,
	0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x01, 0x52, 0x08, 0x4c, 0x61, 0x74, 0x69, 0x74, 0x75,
	0x64, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x4c, 0x6f, 0x6e, 0x67, 0x69, 0x74, 0x75, 0x64, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x01, 0x52, 0x09, 0x4c, 0x6f, 0x6e, 0x67, 0x69, 0x74, 0x75, 0x64, 0x65,
	0x12, 0x1a, 0x0a, 0x08, 0x54, 0x69, 0x6d, 0x65, 0x5a, 0x6f, 0x6e, 0x65, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x08, 0x54, 0x69, 0x6d, 0x65, 0x5a, 0x6f, 0x6e, 0x65, 0x22, 0x93, 0x01, 0x0a,
	0x07, 0x47, 0x65, 0x6f, 0x44, 0x61, 0x74, 0x61, 0x12, 0x2e, 0x0a, 0x07, 0x43, 0x6f, 0x75, 0x6e,
	0x74, 0x72, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x6e, 0x6f, 0x72, 0x6d,
	0x61, 0x6c, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x52,
	0x07, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x25, 0x0a, 0x04, 0x43, 0x69, 0x74, 0x79,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x6e, 0x6f, 0x72, 0x6d, 0x61, 0x6c, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x43, 0x69, 0x74, 0x79, 0x52, 0x04, 0x43, 0x69, 0x74, 0x79, 0x12,
	0x31, 0x0a, 0x08, 0x4c, 0x6f, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x15, 0x2e, 0x6e, 0x6f, 0x72, 0x6d, 0x61, 0x6c, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e,
	0x4c, 0x6f, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x08, 0x4c, 0x6f, 0x63, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x22, 0x9c, 0x01, 0x0a, 0x0c, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x53, 0x74,
	0x61, 0x74, 0x73, 0x12, 0x28, 0x0a, 0x0f, 0x4f, 0x70, 0x65, 0x6e, 0x43, 0x6f, 0x6e, 0x6e, 0x65,
	0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0f, 0x4f, 0x70,
	0x65, 0x6e, 0x43, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x2c, 0x0a,
	0x11, 0x43, 0x6c, 0x6f, 0x73, 0x65, 0x64, 0x43, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x11, 0x43, 0x6c, 0x6f, 0x73, 0x65, 0x64,
	0x43, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x42,
	0x79, 0x74, 0x65, 0x73, 0x49, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x42, 0x79,
	0x74, 0x65, 0x73, 0x49, 0x6e, 0x12, 0x1a, 0x0a, 0x08, 0x42, 0x79, 0x74, 0x65, 0x73, 0x4f, 0x75,
	0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x42, 0x79, 0x74, 0x65, 0x73, 0x4f, 0x75,
	0x74, 0x42, 0x0c, 0x5a, 0x0a, 0x61, 0x6e, 0x61, 0x6c, 0x79, 0x74, 0x69, 0x63, 0x73, 0x2f, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_analytics_proto_rawDescOnce sync.Once
	file_analytics_proto_rawDescData = file_analytics_proto_rawDesc
)

func file_analytics_proto_rawDescGZIP() []byte {
	file_analytics_proto_rawDescOnce.Do(func() {
		file_analytics_proto_rawDescData = protoimpl.X.CompressGZIP(file_analytics_proto_rawDescData)
	})
	return file_analytics_proto_rawDescData
}

var file_analytics_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_analytics_proto_goTypes = []interface{}{
	(*AnalyticsRecord)(nil),       // 0: normalproto.AnalyticsRecord
	(*Latency)(nil),               // 1: normalproto.Latency
	(*Country)(nil),               // 2: normalproto.Country
	(*City)(nil),                  // 3: normalproto.City
	(*Location)(nil),              // 4: normalproto.Location
	(*GeoData)(nil),               // 5: normalproto.GeoData
	(*NetworkStats)(nil),          // 6: normalproto.NetworkStats
	nil,                           // 7: normalproto.City.NamesEntry
	(*timestamppb.Timestamp)(nil), // 8: google.protobuf.Timestamp
}
var file_analytics_proto_depIdxs = []int32{
	8, // 0: normalproto.AnalyticsRecord.TimeStamp:type_name -> google.protobuf.Timestamp
	1, // 1: normalproto.AnalyticsRecord.Latency:type_name -> normalproto.Latency
	5, // 2: normalproto.AnalyticsRecord.Geo:type_name -> normalproto.GeoData
	6, // 3: normalproto.AnalyticsRecord.Network:type_name -> normalproto.NetworkStats
	8, // 4: normalproto.AnalyticsRecord.ExpireAt:type_name -> google.protobuf.Timestamp
	7, // 5: normalproto.City.Names:type_name -> normalproto.City.NamesEntry
	2, // 6: normalproto.GeoData.Country:type_name -> normalproto.Country
	3, // 7: normalproto.GeoData.City:type_name -> normalproto.City
	4, // 8: normalproto.GeoData.Location:type_name -> normalproto.Location
	9, // [9:9] is the sub-list for method output_type
	9, // [9:9] is the sub-list for method input_type
	9, // [9:9] is the sub-list for extension type_name
	9, // [9:9] is the sub-list for extension extendee
	0, // [0:9] is the sub-list for field type_name
}

func init() { file_analytics_proto_init() }
func file_analytics_proto_init() {
	if File_analytics_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_analytics_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AnalyticsRecord); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_analytics_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Latency); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_analytics_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Country); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_analytics_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*City); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_analytics_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Location); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_analytics_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GeoData); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_analytics_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NetworkStats); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_analytics_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_analytics_proto_goTypes,
		DependencyIndexes: file_analytics_proto_depIdxs,
		MessageInfos:      file_analytics_proto_msgTypes,
	}.Build()
	File_analytics_proto = out.File
	file_analytics_proto_rawDesc = nil
	file_analytics_proto_goTypes = nil
	file_analytics_proto_depIdxs = nil
}
