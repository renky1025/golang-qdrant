package qdrantUtils

import (
	"context"
	"strings"

	pb "github.com/qdrant/go-client/qdrant"
	"github.com/webws/go-moda/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	ErrNotFound      = "Not found"
	ErrAlreadyExists = "already exists"
)

// QdrantClient 封装 Qdrant gRPC 客户端及常用参数
// collection: 集合名，size: 向量维度
// 使用 NewQdrantClient 创建实例
// 用完请调用 Close 释放连接
type QdrantClient struct {
	grpcConn   *grpc.ClientConn
	collection string
	size       uint64
}

// Close 关闭 gRPC 连接
func (qc *QdrantClient) Close() {
	qc.grpcConn.Close()
}

// Collection 获取集合管理客户端
func (qc *QdrantClient) Collection() pb.CollectionsClient {
	return pb.NewCollectionsClient(qc.grpcConn)
}

// NewQdrantClient 创建 QdrantClient 实例
// qdrantAddr: Qdrant gRPC 地址
// collection: 集合名
// size: 向量维度
func NewQdrantClient(qdrantAddr, collection string, size uint64) *QdrantClient {
	conn, err := grpc.Dial(qdrantAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatalw("did not connect", "err", err)
	}
	return &QdrantClient{grpcConn: conn, collection: collection, size: size}
}

// toPayload 辅助方法，将 map[string]string 转为 Qdrant Payload
func toPayload(payload map[string]string) map[string]*pb.Value {
	ret := make(map[string]*pb.Value)
	for k, v := range payload {
		ret[k] = &pb.Value{Kind: &pb.Value_StringValue{StringValue: v}}
	}
	return ret
}

// DeleteCollection 删除指定集合
func (qc *QdrantClient) DeleteCollection(name string) error {
	cc := pb.NewCollectionsClient(qc.grpcConn)
	_, err := cc.Delete(context.TODO(), &pb.DeleteCollection{
		CollectionName: name,
	})
	return err
}

// CreateCollection 创建集合（如已存在则忽略）
func (qc *QdrantClient) CreateCollection(name string, size uint64) error {
	cc := pb.NewCollectionsClient(qc.grpcConn)
	req := &pb.CreateCollection{
		CollectionName: name,
		VectorsConfig: &pb.VectorsConfig{
			Config: &pb.VectorsConfig_Params{
				Params: &pb.VectorParams{
					Size:     size,
					Distance: pb.Distance_Cosine,
				},
			},
		},
	}
	_, err := cc.Create(context.Background(), req)
	if err != nil && strings.Contains(err.Error(), ErrAlreadyExists) {
		return nil
	}
	if err != nil {
		logger.Errorw("CreateCollection", "err", err)
		return err
	}
	return nil
}

// CreatePoints 批量插入点（向量）
func (qc *QdrantClient) CreatePoints(points []*pb.PointStruct) error {
	pc := pb.NewPointsClient(qc.grpcConn)

	wait := true
	pointsReq := pb.UpsertPoints{
		CollectionName: qc.collection,
		Points:         points,
		Wait:           &wait,
	}

	_, err := pc.Upsert(context.TODO(), &pointsReq)
	if err != nil {
		logger.Errorw("CreatePoints fail", "err", err)
		return err
	}
	return nil
}

// CreatePoint 插入单个点（向量）
// uuid: 点ID，collection: 集合名，vector: 向量，payload: 附加信息
func (qc *QdrantClient) CreatePoint(uuid string, collection string, vector []float32, payload map[string]string) error {
	point := &pb.PointStruct{}
	point.Id = &pb.PointId{
		PointIdOptions: &pb.PointId_Uuid{
			Uuid: uuid,
		},
	}
	point.Vectors = &pb.Vectors{
		VectorsOptions: &pb.Vectors_Vector{
			Vector: &pb.Vector{
				Data: vector,
			},
		},
	}
	point.Payload = toPayload(payload)

	pc := pb.NewPointsClient(qc.grpcConn)

	wait := true
	points := pb.UpsertPoints{
		CollectionName: collection,
		Points:         []*pb.PointStruct{point},
		Wait:           &wait,
	}

	_, err := pc.Upsert(context.TODO(), &points)
	if err != nil {
		return err
	}
	return nil
}

// Search 搜索相似向量，返回分数最高的点
// vector: 查询向量
func (qc *QdrantClient) Search(vector []float32) ([]*pb.ScoredPoint, error) {
	sc := pb.NewPointsClient(qc.grpcConn)
	rsp, err := sc.Search(context.Background(), &pb.SearchPoints{
		CollectionName: qc.collection,
		Vector:         vector,
		Limit:          3, // 只取前三个
		WithPayload: &pb.WithPayloadSelector{
			SelectorOptions: &pb.WithPayloadSelector_Include{
				Include: &pb.PayloadIncludeSelector{
					Fields: []string{"question", "answers"},
				},
			},
		},
	})
	if err != nil && strings.Contains(err.Error(), ErrNotFound) {
		if err := qc.CreateCollection(qc.collection, qc.size); err != nil {
			logger.Errorw("Search CreateCollection fail", "err", err)
			return nil, err
		}
		return qc.Search(vector)
	}

	if err != nil {
		return nil, err
	}

	return rsp.Result, nil
}
