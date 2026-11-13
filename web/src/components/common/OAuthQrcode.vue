<template>
  <div class="oauth-qrcode">
    <div class="oauth-qrcode-frame">
      <div :id="containerId" class="oauth-qrcode-sdk"></div>
      <!--
        The mask must stay outside the SDK container: both SDKs wipe the element they render into,
        so anything placed inside would be dropped on the next render.
      -->
      <div v-if="expired" class="oauth-qrcode-mask">
        <p class="oauth-qrcode-mask-text">{{ $t("oauth_qrcodeExpired") }}</p>
        <button
          type="button"
          class="oauth-qrcode-refresh"
          @click="handleRefresh"
        >
          <ReloadOutlined class="oauth-qrcode-refresh-icon" />
          <span>{{ $t("oauth_qrcodeRefresh") }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ReloadOutlined } from "@ant-design/icons-vue";

// The two providers ship different official SDKs, so the rendering is split by provider.
// Both SDKs end up navigating the browser to the redirect URI we registered with the provider:
// DingTalk hands back a ready-made redirectUrl, Feishu hands back a tmp_code to append to goto.
const props = defineProps({
  /** Provider key: dingtalk or feishu. */
  provider: { type: String, required: true },
  /** Authorize payload returned by the backend (sdk_url / client_id / authorize_url / redirect_uri / state). */
  authorize: { type: Object, default: null },
  /** Tip shown under the QR code. */
  tip: { type: String, default: "" },
  /** QR code lifetime in seconds, handed down by the backend; 0 turns the expiry mask off. */
  expiresIn: { type: Number, default: 0 },
});

const emit = defineEmits(["error", "refresh", "ready"]);

const containerId = `oauth-qrcode-${Math.random().toString(36).slice(2, 10)}`;
// The QR code is rendered once per state; re-rendering on unrelated updates would reload the iframe.
const renderedState = ref("");
let messageHandler = null;
let readyObserver = null;
let readyIframe = null;
let readyHandler = null;

// Neither SDK reports "nobody scanned it in time": DingTalk and Feishu only message the parent
// page after a scan succeeds or fails. The countdown therefore runs on our side and drives the
// mask, while a failed scan (a code that already died) flips it immediately.
const expired = ref(false);
let expireTimer = null;

// Load each SDK script at most once per page.
const scriptPromises = new Map();

function loadScript(url) {
  if (scriptPromises.has(url)) {
    return scriptPromises.get(url);
  }
  const promise = new Promise((resolve, reject) => {
    if (document.querySelector(`script[src="${url}"]`)) {
      resolve();
      return;
    }
    const script = document.createElement("script");
    script.src = url;
    script.async = true;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error(`load oauth sdk failed: ${url}`));
    document.head.appendChild(script);
  });
  scriptPromises.set(url, promise);
  return promise;
}

function clearExpireTimer() {
  if (expireTimer) {
    clearTimeout(expireTimer);
    expireTimer = null;
  }
}

function startExpireTimer() {
  clearExpireTimer();
  if (props.expiresIn <= 0) {
    return;
  }
  expireTimer = setTimeout(() => {
    expireTimer = null;
    expired.value = true;
  }, props.expiresIn * 1000);
}

// Hand the reload to the parent: only it can fetch a new authorize payload (and a new state).
// The mask stays up until that payload arrives, so the dead code cannot be scanned again.
function handleRefresh() {
  emit("refresh");
}

function detachMessageListener() {
  if (messageHandler) {
    window.removeEventListener("message", messageHandler, false);
    messageHandler = null;
  }
}

function detachReadyListener() {
  readyObserver?.disconnect();
  readyObserver = null;
  if (readyIframe && readyHandler) {
    readyIframe.removeEventListener("load", readyHandler);
  }
  readyIframe = null;
  readyHandler = null;
}

// Both provider SDKs create their iframe themselves, sometimes after the render call returns.
function emitReadyAfterIframeLoad() {
  const container = document.getElementById(containerId);
  if (!container) return;

  const bindLoadEvent = () => {
    const iframe = container.querySelector("iframe");
    if (!iframe) return false;
    readyIframe = iframe;
    readyHandler = () => {
      detachReadyListener();
      emit("ready");
    };
    iframe.addEventListener("load", readyHandler, { once: true });
    return true;
  };

  if (bindLoadEvent()) return;
  readyObserver = new MutationObserver(() => {
    if (bindLoadEvent()) readyObserver?.disconnect();
  });
  readyObserver.observe(container, { childList: true, subtree: true });
}

function renderDingtalk(authorize) {
  if (typeof window.DTFrameLogin !== "function") {
    emit("error", new Error("dingtalk sdk is unavailable"));
    return;
  }
  window.DTFrameLogin(
    { id: containerId, width: 300, height: 300 },
    {
      redirect_uri: encodeURIComponent(authorize.redirect_uri),
      response_type: "code",
      client_id: authorize.client_id,
      scope: "openid",
      state: authorize.state,
      prompt: "consent",
    },
    (loginResult) => {
      // DingTalk returns a ready-made redirectUrl; following it lands on our callback endpoint.
      if (loginResult && loginResult.redirectUrl) {
        window.location.href = loginResult.redirectUrl;
      }
    },
    (errorMessage) => {
      // A failed scan is almost always a code that already expired. The mask says exactly that and
      // offers the way out, so no extra error toast is raised here.
      expired.value = true;
      if (errorMessage) {
        console.warn(`dingtalk login failed: ${errorMessage}`);
      }
    },
  );
}

function renderFeishu(authorize) {
  if (typeof window.QRLogin !== "function") {
    emit("error", new Error("feishu sdk is unavailable"));
    return;
  }
  const goto = authorize.authorize_url;
  const qrLogin = window.QRLogin({
    id: containerId,
    goto,
    width: "300",
    height: "300",
    style: "border:none;background-color:#FFFFFF;",
  });
  messageHandler = (event) => {
    if (!qrLogin.matchOrigin(event.origin) || !qrLogin.matchData(event.data)) {
      return;
    }
    // Feishu requires tmp_code to be appended to the authorize URL, then the browser is redirected.
    window.location.href = `${goto}&tmp_code=${event.data.tmp_code}`;
  };
  window.addEventListener("message", messageHandler, false);
}

async function render() {
  const authorize = props.authorize;
  if (!authorize || !authorize.sdk_url) {
    return;
  }
  const state = `${props.provider}:${authorize.state || ""}`;
  if (renderedState.value === state) {
    return;
  }
  renderedState.value = state;
  expired.value = false;
  clearExpireTimer();
  detachMessageListener();
  detachReadyListener();

  try {
    await loadScript(authorize.sdk_url);
    await nextTick();
    if (props.provider === "feishu") {
      renderFeishu(authorize);
    } else {
      renderDingtalk(authorize);
    }
    emitReadyAfterIframeLoad();
    startExpireTimer();
  } catch (error) {
    emit("error", error);
  }
}

onMounted(render);
watch(
  () => props.authorize && props.authorize.state,
  () => render(),
);
onBeforeUnmount(() => {
  clearExpireTimer();
  detachMessageListener();
  detachReadyListener();
});
</script>

<style scoped>
.oauth-qrcode {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.oauth-qrcode-frame {
  position: relative;
  width: 300px;
  height: 300px;
}

.oauth-qrcode-sdk {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
}

.oauth-qrcode-mask {
  position: absolute;
  inset: 0;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  background: rgba(255, 255, 255, 0.92);
}

.oauth-qrcode-mask-text {
  margin: 0;
  color: #8c8c8c;
  font-size: 14px;
  line-height: 22px;
}

.oauth-qrcode-refresh {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 16px;
  color: #3157e2;
  font-size: 14px;
  line-height: 22px;
  background: #ffffff;
  border: 1px solid #3157e2;
  border-radius: 6px;
  cursor: pointer;
}

.oauth-qrcode-refresh:hover {
  color: #5573e9;
  border-color: #5573e9;
}

.oauth-qrcode-refresh-icon {
  font-size: 14px;
}

.oauth-qrcode-tip {
  margin: 0;
  color: #8c8c8c;
  font-size: 13px;
  line-height: 20px;
  text-align: center;
}
</style>
