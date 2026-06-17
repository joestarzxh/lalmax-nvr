<script lang="ts">
  import { onDestroy } from 'svelte';
  import { Mic, MicOff, Loader2 } from 'lucide-svelte';
  import { t } from '$lib/i18n';
  import { showToast } from '$lib/toast';
  import { getAuthToken } from '$lib/api/client';

  let { cameraId }: { cameraId: string } = $props();

  let isTalking = $state(false);
  let isConnecting = $state(false);
  let ws: WebSocket | null = null;
  let audioContext: AudioContext | null = null;
  let mediaStream: MediaStream | null = null;
  let processor: ScriptProcessorNode | null = null;

  const SAMPLE_RATE = 8000;
  const BUFFER_SIZE = 1024;

  async function startTalk() {
    if (isTalking || isConnecting) return;
    isConnecting = true;

    try {
      mediaStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          channelCount: 1,
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
        video: false,
      });

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const params = new URLSearchParams({ camera_id: cameraId });
      const authToken = getAuthToken();
      if (authToken) params.set('token', authToken);
      const wsUrl = `${protocol}//${window.location.host}/api/xiaomi/talk/ws?${params.toString()}`;

      ws = new WebSocket(wsUrl);
      ws.binaryType = 'arraybuffer';

      ws.onopen = () => {
        startAudioCapture();
        isTalking = true;
        isConnecting = false;
        showToast(t('live.talk.started') || '对讲已开始', 'success');
      };

      ws.onerror = () => {
        showToast(t('live.talk.error') || '对讲连接失败', 'error');
        stopTalk();
      };

      ws.onclose = () => {
        if (isTalking) {
          showToast(t('live.talk.stopped') || '对讲已停止', 'info');
        }
        stopTalk();
      };

      setTimeout(() => {
        if (isConnecting) {
          showToast(t('live.talk.timeout') || '连接超时', 'error');
          stopTalk();
        }
      }, 10000);
    } catch (e) {
      if (e instanceof DOMException) {
        if (e.name === 'NotAllowedError') {
          showToast(t('live.talk.permissionDenied') || '麦克风权限被拒绝', 'error');
        } else if (e.name === 'NotFoundError') {
          showToast(t('live.talk.noMicrophone') || '未检测到麦克风设备', 'error');
        } else {
          showToast(t('live.talk.error') || '启动对讲失败', 'error');
        }
      }
      isConnecting = false;
      stopTalk();
    }
  }

  function startAudioCapture() {
    if (!mediaStream || !ws) return;

    audioContext = new AudioContext({ sampleRate: SAMPLE_RATE });
    const source = audioContext.createMediaStreamSource(mediaStream);

    processor = audioContext.createScriptProcessor(BUFFER_SIZE, 1, 1);
    processor.onaudioprocess = (e) => {
      if (!ws || ws.readyState !== WebSocket.OPEN) return;
      const inputData = e.inputBuffer.getChannelData(0);
      const pcmData = float32ToPCMA(inputData);
      ws.send(pcmData);
    };

    source.connect(processor);
    processor.connect(audioContext.destination);
  }

  function float32ToPCMA(float32Array: Float32Array): ArrayBuffer {
    const buffer = new ArrayBuffer(float32Array.length);
    const view = new Uint8Array(buffer);
    for (let i = 0; i < float32Array.length; i++) {
      let sample = Math.max(-1, Math.min(1, float32Array[i]));
      const pcm = sample < 0 ? sample * 0x8000 : sample * 0x7FFF;
      view[i] = linearToALaw(pcm);
    }
    return buffer;
  }

  function linearToALaw(sample: number): number {
    const ALAW_MAX = 0x7fff;
    let pcm = Math.trunc(sample);
    let mask: number;
    if (pcm >= 0) { mask = 0xd5; } else { mask = 0x55; pcm = -pcm - 1; if (pcm < 0) pcm = 0; }
    if (pcm > ALAW_MAX) pcm = ALAW_MAX;
    let compressed: number;
    if (pcm < 256) {
      compressed = pcm >> 4;
    } else {
      let exponent = 7;
      for (let expMask = 0x4000; (pcm & expMask) === 0 && exponent > 0; expMask >>= 1) { exponent--; }
      compressed = (exponent << 4) | ((pcm >> (exponent + 3)) & 0x0f);
    }
    return (compressed ^ mask) & 0xff;
  }

  function stopTalk() {
    isTalking = false;
    isConnecting = false;
    if (processor) { processor.disconnect(); processor = null; }
    if (audioContext) { audioContext.close(); audioContext = null; }
    if (mediaStream) { mediaStream.getTracks().forEach(t => t.stop()); mediaStream = null; }
    if (ws) { ws.close(); ws = null; }
  }

  function toggleTalk() {
    if (isTalking) { stopTalk(); showToast(t('live.talk.stopped') || '对讲已停止', 'info'); }
    else { startTalk(); }
  }

  onDestroy(() => { stopTalk(); });
</script>

<button
  onclick={toggleTalk}
  disabled={isConnecting}
  class="talk-btn {isTalking ? 'talking' : ''} {isConnecting ? 'connecting' : ''}"
  title={isTalking ? (t('live.talk.stop') || '停止对讲') : (t('live.talk.start') || '开始对讲')}
>
  {#if isConnecting}
    <Loader2 size={20} class="animate-spin" />
  {:else if isTalking}
    <MicOff size={20} />
  {:else}
    <Mic size={20} />
  {/if}
</button>

<style>
  .talk-btn {
    width: 40px;
    height: 40px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--color-primary);
    color: white;
    border: none;
    cursor: pointer;
    transition: all 0.2s ease;
  }
  .talk-btn:hover:not(:disabled) { transform: scale(1.05); }
  .talk-btn.talking {
    background: var(--color-danger, #ef4444);
    animation: pulse 1.5s infinite;
  }
  .talk-btn.connecting { background: var(--color-warning, #f59e0b); cursor: wait; }
  .talk-btn:disabled { opacity: 0.7; cursor: not-allowed; }
  @keyframes pulse {
    0%, 100% { box-shadow: 0 2px 8px rgba(239, 68, 68, 0.3); }
    50% { box-shadow: 0 4px 16px rgba(239, 68, 68, 0.5); }
  }
</style>
