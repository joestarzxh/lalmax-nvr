<script lang="ts">
  import { onDestroy } from 'svelte';
  import { Mic, MicOff, Loader2 } from 'lucide-svelte';
  import { t } from '$lib/i18n';
  import { showToast } from '$lib/toast';

  let { deviceId, channelId }: { deviceId: string; channelId: string } = $props();

  let isTalking = $state(false);
  let isConnecting = $state(false);
  let ws: WebSocket | null = null;
  let audioContext: AudioContext | null = null;
  let mediaStream: MediaStream | null = null;
  let processor: ScriptProcessorNode | null = null;
  let audioWorklet: AudioWorkletNode | null = null;

  const SAMPLE_RATE = 8000;
  const BUFFER_SIZE = 1024;

  async function checkMicrophonePermission(): Promise<boolean> {
    try {
      // Check if Permissions API is supported
      if (navigator.permissions && navigator.permissions.query) {
        const result = await navigator.permissions.query({ name: 'microphone' as PermissionName });
        if (result.state === 'denied') {
          showToast(t('live.talk.permissionDenied') || '麦克风权限被拒绝，请在浏览器设置中允许麦克风权限', 'error');
          return false;
        }
      }
      return true;
    } catch {
      // Permissions API not supported, proceed with getUserMedia
      return true;
    }
  }

  async function startTalk() {
    if (isTalking || isConnecting) return;

    // Check permission first
    const hasPermission = await checkMicrophonePermission();
    if (!hasPermission) return;

    isConnecting = true;
    try {
      // Request microphone permission
      mediaStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          sampleRate: SAMPLE_RATE,
          channelCount: 1,
          echoCancellation: true,
          noiseSuppression: true,
        }
      });

      // Create WebSocket connection
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/api/gb28181/talk/ws?device_id=${deviceId}&channel_id=${channelId}`;
      ws = new WebSocket(wsUrl);
      ws.binaryType = 'arraybuffer';

      ws.onopen = () => {
        // Start audio capture
        startAudioCapture();
        isTalking = true;
        isConnecting = false;
        showToast(t('live.talk.started') || '对讲已开始', 'success');
      };

      ws.onerror = (e) => {
        console.error('WebSocket error:', e);
        showToast(t('live.talk.error') || '对讲连接失败', 'error');
        stopTalk();
      };

      ws.onclose = () => {
        if (isTalking) {
          showToast(t('live.talk.stopped') || '对讲已停止', 'info');
        }
        stopTalk();
      };

      // Timeout
      setTimeout(() => {
        if (isConnecting) {
          showToast(t('live.talk.timeout') || '连接超时', 'error');
          stopTalk();
        }
      }, 10000);

    } catch (e) {
      console.error('Failed to start talk:', e);
      if (e instanceof DOMException && e.name === 'NotAllowedError') {
        showToast(t('live.talk.permissionDenied') || '请允许麦克风权限', 'error');
      } else if (e instanceof DOMException && e.name === 'NotFoundError') {
        showToast(t('live.talk.noMicrophone') || '未检测到麦克风设备', 'error');
      } else {
        showToast(t('live.talk.error') || '启动对讲失败', 'error');
      }
      isConnecting = false;
    }
  }

  function startAudioCapture() {
    if (!mediaStream || !ws) return;

    audioContext = new AudioContext({ sampleRate: SAMPLE_RATE });
    const source = audioContext.createMediaStreamSource(mediaStream);

    // Use ScriptProcessorNode for broader compatibility
    processor = audioContext.createScriptProcessor(BUFFER_SIZE, 1, 1);
    processor.onaudioprocess = (e) => {
      if (!ws || ws.readyState !== WebSocket.OPEN) return;

      const inputData = e.inputBuffer.getChannelData(0);
      // Convert float32 to PCM16 (G.711)
      const pcmData = float32ToPCM16(inputData);
      ws.send(pcmData);
    };

    source.connect(processor);
    processor.connect(audioContext.destination);
  }

  function float32ToPCM16(float32Array: Float32Array): ArrayBuffer {
    const buffer = new ArrayBuffer(float32Array.length * 2);
    const view = new DataView(buffer);
    for (let i = 0; i < float32Array.length; i++) {
      let sample = float32Array[i];
      // Clamp
      sample = Math.max(-1, Math.min(1, sample));
      // Convert to 16-bit PCM
      view.setInt16(i * 2, sample < 0 ? sample * 0x8000 : sample * 0x7FFF, true);
    }
    return buffer;
  }

  function stopTalk() {
    isTalking = false;
    isConnecting = false;

    // Stop audio processing
    if (processor) {
      processor.disconnect();
      processor = null;
    }

    if (audioContext) {
      audioContext.close();
      audioContext = null;
    }

    // Stop microphone
    if (mediaStream) {
      mediaStream.getTracks().forEach(track => track.stop());
      mediaStream = null;
    }

    // Close WebSocket
    if (ws) {
      ws.close();
      ws = null;
    }
  }

  function toggleTalk() {
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
  <!-- Talk button -->
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

  .audio-indicator .bar:nth-child(1) {
    animation-delay: 0s;
    height: 8px;
  }
  .audio-indicator .bar:nth-child(2) {
    animation-delay: 0.1s;
    height: 16px;
  }
  .audio-indicator .bar:nth-child(3) {
    animation-delay: 0.2s;
    height: 12px;
  }
  .audio-indicator .bar:nth-child(4) {
    animation-delay: 0.3s;
    height: 18px;
  }
  .audio-indicator .bar:nth-child(5) {
    animation-delay: 0.4s;
    height: 10px;
  }

  @keyframes audioBar {
    0% {
      height: 4px;
    }
    100% {
      height: 20px;
    }
  }
</style>
