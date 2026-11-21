import { ref, onScopeDispose } from 'vue';
import { post } from '@/api/client';
import { logger } from '@/utils/logger';
import i18n from '@/i18n';

const { t } = i18n.global;

// useVoice 提供语音交互能力：录音转写、文本朗读。
// 依赖浏览器 MediaRecorder 与后端 /ai/voice/transcribe、/ai/voice/synthesize。
export function useVoice() {
  const recording = ref(false);
  const speaking = ref(false);
  const transcript = ref('');
  const error = ref('');
  let mediaStream: MediaStream | null = null;
  let recorder: MediaRecorder | null = null;
  let chunks: BlobPart[] = [];
  let audioEl: HTMLAudioElement | null = null;

  async function startRecording(): Promise<void> {
    error.value = '';
    try {
      mediaStream = await navigator.mediaDevices.getUserMedia({ audio: true });
      recorder = new MediaRecorder(mediaStream);
      chunks = [];
      recorder.ondataavailable = (e) => { if (e.data.size > 0) chunks.push(e.data); };
      recorder.onstop = async () => {
        const blob = new Blob(chunks, { type: recorder?.mimeType || 'audio/webm' });
        await sendTranscribe(blob);
        stopStream();
      };
      recorder.start();
      recording.value = true;
    } catch (e) {
      logger.error('无法访问麦克风', e);
      error.value = t('tsUi.k3') + (e as Error).message;
      recording.value = false;
    }
  }

  function stopRecording(): void {
    if (recorder && recorder.state !== 'inactive') {
      recorder.stop();
    }
    recording.value = false;
  }

  async function sendTranscribe(blob: Blob): Promise<void> {
    const form = new FormData();
    form.append('audio', blob, 'recording.webm');
    try {
      const res = await post<{ text: string }>('/ai/voice/transcribe', form, {
        headers: { 'Content-Type': 'multipart/form-data' },
        timeout: 120000,
      });
      transcript.value = res.text || '';
    } catch (e) {
      logger.error('语音转写失败', e);
      error.value = t('tsUi.k4') + (e as Error).message;
    }
  }

  async function speak(text: string, voice = 'alloy'): Promise<void> {
    try {
      const res = await post<Blob>('/ai/voice/synthesize', { text, voice }, {
        responseType: 'blob',
        timeout: 60000,
      });
      const url = URL.createObjectURL(res);
      if (!audioEl) audioEl = new Audio();
      if (audioEl.src) URL.revokeObjectURL(audioEl.src);
      audioEl.onended = () => {
        speaking.value = false;
        if (audioEl?.src) URL.revokeObjectURL(audioEl.src);
      };
      audioEl.src = url;
      speaking.value = true;
      await audioEl.play();
    } catch (e) {
      logger.error('语音合成失败', e);
      error.value = t('tsUi.k5') + (e as Error).message;
      speaking.value = false;
    }
  }

  function stopStream(): void {
    if (mediaStream) {
      mediaStream.getTracks().forEach((t) => t.stop());
      mediaStream = null;
    }
  }

  function stopSpeaking(): void {
    if (audioEl) audioEl.pause();
    speaking.value = false;
  }

  onScopeDispose(() => {
    stopStream();
    stopSpeaking();
    if (audioEl?.src) {
      URL.revokeObjectURL(audioEl.src);
      audioEl.src = '';
    }
  });

  return { recording, speaking, transcript, error, startRecording, stopRecording, speak, stopSpeaking };
}
