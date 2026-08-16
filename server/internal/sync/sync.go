// Package sync 云同步核心: Vault 上传/拉取 + 版本冲突处理 (零知识)
//
// 规则 (docs/05 第 2-4 节):
//   R2  上传仅接受 clientVersion == 当前版本, 成功后版本 +1
//   R7  冲突由客户端解决 (服务端返回最新版本, 不做内容合并)
//   R8  同版本重复上传视为幂等重试, 直接返回成功
//   R12 版本检查在单事务内原子完成 (store.PutVault)
package sync

import (
	"errors"
	"time"

	"ssh-terminal/server/internal/store"
)

// ErrVaultConflict 版本冲突: 客户端版本落后于服务端
var ErrVaultConflict = errors.New("vault 版本冲突")

// Service 同步服务
type Service struct {
	st *store.Store
}

// New 创建同步服务
func New(st *store.Store) *Service {
	return &Service{st: st}
}

// Pull 拉取用户 Vault (零知识: 密文原样返回)
func (s *Service) Pull(userID string) (store.VaultRow, error) {
	return s.st.GetVault(userID)
}

// Push 上传新版本 Vault:
//   - 客户端版本落后 → 返回 ErrVaultConflict (调用方需返回最新版本供合并)
//   - 同版本重复上传 (幂等重试) → 直接成功
// 返回新版本号
func (s *Service) Push(userID string, clientVersion int64, ciphertext string) (int64, error) {
	newVersion, ok, err := s.st.PutVault(userID, clientVersion, ciphertext)
	if err != nil {
		return 0, err
	}
	if !ok {
		// 冲突: 取最新版本返回给客户端做合并
		cur, _ := s.st.GetVault(userID)
		return cur.Version, ErrVaultConflict
	}
	return newVersion, nil
}

// MergeLocalWins 客户端"用我的"策略: 强制以本地为最新 (服务端回滚/重建场景)
// 将本地密文直接写为新版本 (版本 = 服务端当前 + 1)
func (s *Service) MergeLocalWins(userID string, ciphertext string) (int64, error) {
	cur, err := s.st.GetVault(userID)
	if err != nil {
		return 0, err
	}
	return s.Push(userID, cur.Version, ciphertext)
}

// TouchDevice 更新设备最近活跃时间
func (s *Service) TouchDevice(deviceID string) {
	d, err := s.st.GetDevice(deviceID)
	if err != nil {
		return
	}
	d.LastSeen = time.Now().Format(time.RFC3339)
	_ = s.st.UpdateDevice(d)
}
