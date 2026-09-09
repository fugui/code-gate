package routing

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"code-gate/internal/config"
	"code-gate/internal/models"
	"code-gate/internal/store"
)

var (
	ErrNoBackendAvailable  = errors.New("目标模型无健康可用的物理后端实例")
	ErrNoCompatibleBackend = errors.New("目标模型下无物理实例支持所请求的协议端点")
	ErrModelNotFound       = errors.New("目标模型不存在或未启用")
)

const (
	StrategyWeightedLeastConn  = "least_conn"
	StrategyWeightedRoundRobin = "round_robin"
)

// Router 协议感知智能路由器
type Router struct {
	rrCounters sync.Map // map[uint]*uint32 记录每个模型的轮询游标
}

// NewRouter 创建路由器实例
func NewRouter() *Router {
	return &Router{}
}

// SelectBackend 根据会话标识、请求协议类型和负载均衡策略，为模型选取最优的物理后端
func (r *Router) SelectBackend(
	model *models.Model,
	requestedProtocol string,
	sessionID string,
	strategy string,
) (*models.Backend, error) {
	if model == nil {
		return nil, ErrModelNotFound
	}

	// 1. 协议感知前置过滤：筛选出既健康、又明确支持该协议的物理后端
	var compatibleBackends []*models.Backend
	for i := range model.Backends {
		b := &model.Backends[i]
		if b.IsHealthy && b.SupportsProtocol(requestedProtocol) {
			compatibleBackends = append(compatibleBackends, b)
		}
	}

	// 2. 若当前模型下无兼容节点，尝试触发默认模型 Fallback 容灾降级
	if len(compatibleBackends) == 0 {
		cfg := config.Get()
		if cfg != nil && cfg.Defaults.DefaultModel != "" && cfg.Defaults.DefaultModel != model.Name {
			defaultModelName := cfg.Defaults.DefaultModel
			db := store.GetDB()
			if db != nil {
				var fallbackModel models.Model
				if err := db.Preload("Backends").Where("name = ? AND is_enabled = ?", defaultModelName, true).First(&fallbackModel).Error; err == nil {
					// 递归尝试在降级模型中选路
					for i := range fallbackModel.Backends {
						b := &fallbackModel.Backends[i]
						if b.IsHealthy && b.SupportsProtocol(requestedProtocol) {
							compatibleBackends = append(compatibleBackends, b)
						}
					}
					if len(compatibleBackends) > 0 {
						model = &fallbackModel
					}
				}
			}
		}
	}

	if len(compatibleBackends) == 0 {
		// 区分无可用后端还是无兼容协议后端
		if len(model.Backends) > 0 {
			return nil, fmt.Errorf("%w: 模型 [%s] 的实例均不支持协议 [%s]", ErrNoCompatibleBackend, model.Name, requestedProtocol)
		}
		return nil, fmt.Errorf("%w: 模型 [%s]", ErrNoBackendAvailable, model.Name)
	}

	if len(compatibleBackends) == 1 {
		return compatibleBackends[0], nil
	}

	// 3. 若携带有效会话标识，优先执行 HRW KV Cache 会话亲和性调度 (含溢出保护)
	if sessionID != "" {
		if b := SelectByHRW(sessionID, compatibleBackends); b != nil {
			return b, nil
		}
	}

	// 4. 执行常规负载均衡调度
	if strategy == StrategyWeightedRoundRobin {
		return r.selectByRoundRobin(model.ID, compatibleBackends), nil
	}
	// 默认使用加权最少连接
	return r.selectByLeastConn(compatibleBackends), nil
}

// selectByLeastConn 加权最少连接算法：分发至当前 ActiveConnections / Weight 比值最低的实例
func (r *Router) selectByLeastConn(backends []*models.Backend) *models.Backend {
	var best *models.Backend
	minScore := math.MaxFloat64

	for _, b := range backends {
		weight := float64(b.Weight)
		if weight <= 0 {
			weight = 1.0
		}
		active := float64(atomic.LoadInt32(&b.ActiveConnections))
		// 计算负载比率
		score := active / weight

		if score < minScore {
			minScore = score
			best = b
		}
	}

	if best == nil && len(backends) > 0 {
		best = backends[0]
	}
	return best
}

// selectByRoundRobin 平滑轮询调度
func (r *Router) selectByRoundRobin(modelID uint, backends []*models.Backend) *models.Backend {
	val, _ := r.rrCounters.LoadOrStore(modelID, new(uint32))
	counter := val.(*uint32)

	idx := atomic.AddUint32(counter, 1) - 1
	return backends[int(idx)%len(backends)]
}
