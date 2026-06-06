// Copyright 2022, Chef.  All rights reserved.
// https://github.com/q191201771/lal
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package logic

import (
	"time"

	"github.com/q191201771/lal/pkg/hls"
)

func (group *Group) IsHlsMuxerAlive() bool {
	group.mutex.Lock()
	defer group.mutex.Unlock()
	return group.hlsMuxer != nil
}

func (group *Group) isHlsOnDemand() bool {
	if !group.config.HlsConfig.Enable && !group.config.HlsConfig.EnableHttps {
		return false
	}
	return group.config.HlsConfig.OnDemand
}

// startHlsIfNeeded 必要时启动hls
func (group *Group) startHlsIfNeeded() {
	if !group.config.HlsConfig.Enable && !group.config.HlsConfig.EnableHttps {
		return
	}
	if group.isHlsOnDemand() {
		return
	}
	group.startHlsMuxerLocked()
}

func (group *Group) startHlsMuxerLocked() {
	if group.hlsMuxer != nil {
		return
	}
	group.hlsMuxer = hls.NewMuxer(group.streamName, &group.config.HlsConfig.MuxerConfig, group)
	group.hlsMuxer.Start()
	group.hlsLastAccessUnix = time.Now().Unix()
}

func (group *Group) touchHlsAccessLocked() {
	group.hlsLastAccessUnix = time.Now().Unix()
}

// OnHlsAccess is called when an HLS HTTP client requests m3u8 or ts content.
func (group *Group) OnHlsAccess() {
	if !group.config.HlsConfig.Enable && !group.config.HlsConfig.EnableHttps {
		return
	}
	group.mutex.Lock()
	defer group.mutex.Unlock()
	group.touchHlsAccessLocked()
	if group.isHlsOnDemand() && group.hlsMuxer == nil && group.hasInSession() {
		group.startHlsMuxerLocked()
	}
}

func (group *Group) tickHlsOnDemandIdleLocked() {
	if !group.isHlsOnDemand() || group.hlsMuxer == nil {
		return
	}
	idleMs := group.config.HlsConfig.OnDemandIdleTimeoutMs
	if idleMs <= 0 {
		idleMs = 60000
	}
	if time.Now().Unix()-group.hlsLastAccessUnix >= int64(idleMs)/1000 {
		group.stopHlsMuxerLocked()
	}
}

func (group *Group) stopHlsIfNeeded() {
	if !group.config.HlsConfig.Enable && !group.config.HlsConfig.EnableHttps {
		return
	}
	group.mutex.Lock()
	defer group.mutex.Unlock()
	group.stopHlsMuxerLocked()
}

func (group *Group) stopHlsMuxerLocked() {
	if group.hlsMuxer == nil {
		return
	}
	group.hlsMuxer.Dispose()
	group.observer.CleanupHlsIfNeeded(group.appName, group.streamName, group.hlsMuxer.OutPath())
	group.hlsMuxer = nil
}
