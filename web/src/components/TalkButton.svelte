<script lang="ts">
  import { onDestroy } from 'svelte';
  import { Mic, MicOff, Loader2 } from 'lucide-svelte';
  import { t } from '$lib/i18n';
  import { showToast } from '$lib/toast';
  import { getAuthToken } from '$lib/api/client';

  let { deviceId, channelId }: { deviceId: string; channelId: string } = $props();

  let isTalking = $state(false);
  let isConnecting = $state(false);
  let ws: WebSocket | null = null;
  let audioContext: AudioContext | null = null;
  let mediaStream: MediaStream | null = null;
  let processor: ScriptProcessorNode | null = null;

  const SAMPLE_RATE = 8000;
  const BUFFER_SIZE = 1024;

  async function startTalk() {
    console.log('[Talk] startTalk called', { isTalking, isConnecting, deviceId, channelId });
    if (isTalking || isConnecting) return;

    isConnecting = true;
    try {
      console.log('[Talk] Requesting microphone...');
      // 获取音频流，使用WebRTC的音频处理（回声消除、降噪等）
      mediaStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          channelCount: 1,
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
        video: false,
      });
      console.log('[Talk] Got media stream:', mediaStream);

      // 创建WebSocket连接
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const params = new URLSearchParams({
        device_id: deviceId,
        channel_id: channelId,
      });
      const authToken = getAuthToken();
      if (authToken) params.set('token', authToken);
      const wsUrl = `${protocol}//${window.location.host}/api/gb28181/talk/ws?${params.toString()}`;
      
      console.log('[Talk] Connecting WebSocket to:', wsUrl);
      ws = new WebSocket(wsUrl);
      ws.binaryType = 'arraybuffer';

      ws.onopen = () => {
        console.log('[Talk] WebSocket connected');
        startAudioCapture();
        isTalking = true;
        isConnecting = false;
        showToast(t('live.talk.started') || '对讲已开始', 'success');
      };

      ws.onerror = (e) => {
        console.error('[Talk] WebSocket error:', e);
        showToast(t('live.talk.error') || '对讲连接失败', 'error');
        stopTalk();
      };

      ws.onclose = (e) => {
        console.log('[Talk] WebSocket closed:', e.code, e.reason);
        if (isTalking) {
          showToast(t('live.talk.stopped') || '对讲已停止', 'info');
        }
        stopTalk();
      };

      // 超时处理
      setTimeout(() => {
        if (isConnecting) {
          console.warn('[Talk] Connection timeout');
          showToast(t('live.talk.timeout') || '连接超时', 'error');
          stopTalk();
        }
      }, 10000);

    } catch (e) {
      console.error('[Talk] Failed to start:', e);
      if (e instanceof DOMException) {
        if (e.name === 'NotAllowedError') {
          showToast(t('live.talk.permissionDenied') || '麦克风权限被拒绝，请点击地址栏左侧的麦克风图标允许权限', 'error');
        } else if (e.name === 'NotFoundError') {
          showToast(t('live.talk.noMicrophone') || '未检测到麦克风设备', 'error');
        } else if (e.name === 'NotReadableError') {
          showToast(t('live.talk.microphoneInUse') || '麦克风被其他应用占用', 'error');
        } else {
          showToast(t('live.talk.error') || '启动对讲失败: ' + e.message, 'error');
        }
      } else {
        showToast(t('live.talk.error') || '启动对讲失败', 'error');
      }
      isConnecting = false;
      stopTalk();
    }
  }

  function startAudioCapture() {
    if (!mediaStream || !ws) {
      console.error('[Talk] Cannot start audio capture: missing stream or ws');
      return;
    }

    console.log('[Talk] Starting audio capture...');
    // 使用较低的采样率创建AudioContext，浏览器会自动重采样
    audioContext = new AudioContext({ sampleRate: SAMPLE_RATE });
    const source = audioContext.createMediaStreamSource(mediaStream);

    // 使用ScriptProcessorNode采集音频
    processor = audioContext.createScriptProcessor(BUFFER_SIZE, 1, 1);
    processor.onaudioprocess = (e) => {
      if (!ws || ws.readyState !== WebSocket.OPEN) return;

      const inputData = e.inputBuffer.getChannelData(0);
      const pcmData = float32ToPCMA(inputData);
      ws.send(pcmData);
    };

    source.connect(processor);
    // 连接到destination才能触发onaudioprocess
    processor.connect(audioContext.destination);
    console.log('[Talk] Audio capture started');
  }

  function float32ToPCMA(float32Array: Float32Array): ArrayBuffer {
    const buffer = new ArrayBuffer(float32Array.length);
    const view = new Uint8Array(buffer);
    for (let i = 0; i < float32Array.length; i++) {
      let sample = float32Array[i];
      sample = Math.max(-1, Math.min(1, sample));
      const pcm = sample < 0 ? sample * 0x8000 : sample * 0x7FFF;
      view[i] = linearToALaw(pcm);
    }
    return buffer;
  }

  function linearToALaw(sample: number): number {
    const ALAW_MAX = 0x7fff;
    let pcm = Math.trunc(sample);
    let mask: number;

    if (pcm >= 0) {
      mask = 0xd5;
    } else {
      mask = 0x55;
      pcm = -pcm - 1;
      if (pcm < 0) pcm = 0;
    }

    if (pcm > ALAW_MAX) pcm = ALAW_MAX;

    let compressed: number;
    if (pcm < 256) {
      compressed = pcm >> 4;
    } else {
      let exponent = 7;
      for (let expMask = 0x4000; (pcm & expMask) === 0 && exponent > 0; expMask >>= 1) {
        exponent--;
      }
      compressed = (exponent << 4) | ((pcm >> (exponent + 3)) & 0x0f);
    }

    return (compressed ^ mask) & 0xff;
  }

  function stopTalk() {
    console.log('[Talk] Stopping talk...');
    isTalking = false;
    isConnecting = false;

    if (processor) {
      processor.disconnect();
      processor = null;
    }

    if (audioContext) {
      audioContext.close();
      audioContext = null;
    }

    if (mediaStream) {
      mediaStream.getTracks().forEach(track => track.stop());
      mediaStream = null;
    }

    if (ws) {
      ws.close();
      ws = null;
    }
    console.log('[Talk] Talk stopped');
  }

  function toggleTalk() {
    console.log('[Talk] Button clicked', { isTalking, isConnecting });
    if (isTalking) {
      stopTalk();
      showToast(t('live.talk.stopped') || '对讲已停止', 'info');
    } else {
      startTalk();
    }
  }

  onDestroy(() => {
    stopTalk();
  });
</script>

<div class="flex flex-col items-center gap-3">
  <button
    onclick={toggleTalk}
    disabled={isConnecting}
    class="talk-btn {isTalking ? 'talking' : ''} {isConnecting ? 'connecting' : ''}"
    title={isTalking ? (t('live.talk.stop') || '停止对讲') : (t('live.talk.start') || '开始对讲')}
  >
    {#if isConnecting}
      <Loader2 size={24} class="animate-spin" />
    {:else if isTalking}
      <MicOff size={24} />
    {:else}
      <Mic size={24} />
    {/if}
  </button>

  <span class="text-xs th-text-secondary">
    {#if isConnecting}
      {t('live.talk.connecting') || '连接中...'}
    {:else if isTalking}
      {t('live.talk.talking') || '对讲中...'}
    {:else}
      {t('live.talk.holdToTalk') || '点击开始对讲'}
    {/if}
  </span>

  {#if isTalking}
    <div class="audio-indicator">
      <div class="bar"></div>
      <div class="bar"></div>
      <div class="bar"></div>
      <div class="bar"></div>
      <div class="bar"></div>
    </div>
  {/if}
</div>

<style>
  .talk-btn {
    width: 64px;
    height: 64px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--color-primary);
    color: white;
    border: none;
    cursor: pointer;
    transition: all 0.2s ease;
    box-shadow: 0 4px 12px rgba(139, 92, 246, 0.3);
  }

  .talk-btn:hover:not(:disabled) {
    transform: scale(1.05);
    box-shadow: 0 6px 16px rgba(139, 92, 246, 0.4);
  }

  .talk-btn:active:not(:disabled) {
    transform: scale(0.95);
  }

  .talk-btn.talking {
    background: var(--color-danger, #ef4444);
    box-shadow: 0 4px 12px rgba(239, 68, 68, 0.3);
    animation: pulse 1.5s infinite;
  }

  .talk-btn.connecting {
    background: var(--color-warning, #f59e0b);
    cursor: wait;
  }

  .talk-btn:disabled {
    opacity: 0.7;
    cursor: not-allowed;
  }

  @keyframes pulse {
    0%, 100% {
      box-shadow: 0 4px 12px rgba(239, 68, 68, 0.3);
    }
    50% {
      box-shadow: 0 4px 20px rgba(239, 68, 68, 0.5);
    }
  }

  .audio-indicator {
    display: flex;
    align-items: flex-end;
    gap: 3px;
    height: 20px;
  }

  .audio-indicator .bar {
    width: 4px;
    background: var(--color-primary);
    border-radius: 2px;
    animation: audioBar 0.5s infinite alternate;
  }

  .audio-indicator .bar:nth-child(1) { animation-delay: 0s; height: 8px; }
  .audio-indicator .bar:nth-child(2) { animation-delay: 0.1s; height: 16px; }
  .audio-indicator .bar:nth-child(3) { animation-delay: 0.2s; height: 12px; }
  .audio-indicator .bar:nth-child(4) { animation-delay: 0.3s; height: 18px; }
  .audio-indicator .bar:nth-child(5) { animation-delay: 0.4s; height: 10px; }

  @keyframes audioBar {
    0% { height: 4px; }
    100% { height: 20px; }
  }
</style>
