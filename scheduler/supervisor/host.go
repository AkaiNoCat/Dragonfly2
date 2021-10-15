/*
 *     Copyright 2020 The Dragonfly Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package supervisor

import (
	"sync"

	logger "d7y.io/dragonfly/v2/internal/dflog"
	"go.uber.org/atomic"
)

type HostManager interface {
	// Add host
	Add(*Host)
	// Get host
	Get(string) (*Host, bool)
	// Delete host
	Delete(string)
}

type hostManager struct {
	// host map
	*sync.Map
}

func NewHostManager() HostManager {
	return &hostManager{&sync.Map{}}
}

func (m *hostManager) Get(key string) (*Host, bool) {
	host, ok := m.Load(key)
	if !ok {
		return nil, false
	}

	return host.(*Host), ok
}

func (m *hostManager) Add(host *Host) {
	m.Store(host.UUID, host)
}

func (m *hostManager) Delete(key string) {
	m.Map.Delete(key)
}

type Host struct {
	// uuid each time the daemon starts, it will generate a different uuid
	UUID string
	// IP peer host ip
	IP string
	// HostName peer host name
	HostName string
	// RPCPort rpc service port for peer
	RPCPort int32
	// DownloadPort piece downloading port for peer
	DownloadPort int32
	// IsCDN if host type is cdn
	IsCDN bool
	// SecurityDomain security isolation domain for network
	SecurityDomain string
	// Location location path: area|country|province|city|...
	Location string
	// IDC idc where the peer host is located
	IDC string
	// NetTopology network device path
	// according to the user's own network topology definition, the coverage range from large to small, using the | symbol segmentation,
	// Example: switch|router|...
	NetTopology string
	// TODO TotalUploadLoad currentUploadLoad decided by real time client report host info
	TotalUploadLoad int32
	// CurrentUploadLoad is current upload load number
	CurrentUploadLoad atomic.Int32
	// peers info map
	peers *sync.Map
	// host logger
	logger *logger.SugaredLoggerOnWith
}

func NewClientHost(uuid, ip, hostname string, rpcPort, downloadPort int32, securityDomain, location, idc, netTopology string,
	totalUploadLoad int32) *Host {
	return newHost(uuid, ip, hostname, rpcPort, downloadPort, false, securityDomain, location, idc, netTopology, totalUploadLoad)
}

func NewCDNHost(uuid, ip, hostname string, rpcPort, downloadPort int32, securityDomain, location, idc, netTopology string,
	totalUploadLoad int32) *Host {
	return newHost(uuid, ip, hostname, rpcPort, downloadPort, true, securityDomain, location, idc, netTopology, totalUploadLoad)
}

func newHost(uuid, ip, hostname string, rpcPort, downloadPort int32, isCDN bool, securityDomain, location, idc, netTopology string,
	totalUploadLoad int32) *Host {
	if totalUploadLoad == 0 {
		totalUploadLoad = 100
	}

	return &Host{
		UUID:            uuid,
		IP:              ip,
		HostName:        hostname,
		RPCPort:         rpcPort,
		DownloadPort:    downloadPort,
		IsCDN:           isCDN,
		SecurityDomain:  securityDomain,
		Location:        location,
		IDC:             idc,
		NetTopology:     netTopology,
		TotalUploadLoad: totalUploadLoad,
		peers:           &sync.Map{},
		logger:          logger.With("hostUUID", uuid),
	}
}

func (h *Host) AddPeer(peer *Peer) {
	h.peers.Store(peer.ID, peer)
}

func (h *Host) DeletePeer(id string) {
	h.peers.Delete(id)
}

func (h *Host) GetPeer(id string) (*Peer, bool) {
	peer, ok := h.peers.Load(id)
	if !ok {
		return nil, false
	}

	return peer.(*Peer), ok
}

func (h *Host) GetPeersLen() int {
	length := 0
	h.peers.Range(func(_, _ interface{}) bool {
		length++
		return true
	})

	return length
}

func (h *Host) GetUploadLoadPercent() float64 {
	if h.TotalUploadLoad <= 0 {
		return 1.0
	}

	return float64(h.CurrentUploadLoad.Load()) / float64(h.TotalUploadLoad)
}

func (h *Host) GetFreeUploadLoad() int32 {
	return h.TotalUploadLoad - h.CurrentUploadLoad.Load()
}

func (h *Host) Log() *logger.SugaredLoggerOnWith {
	return h.logger
}
