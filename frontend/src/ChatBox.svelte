<script>
  import { Marked } from 'marked';
  const marked = new Marked();
  // Svelte 5 Props 수신
  let { userName, onConfirmSuccess } = $props();

  // Svelte 5 상태 관리 룬 선언
  let inputVal = $state("");
  let chatLogs = $state([]);
  let isPending = $state(false);
  let latestDebugInfo = $state(null);
  let activeFactContext = $state("");
  let traceSteps = $state([]);
  let pendingConfirm = $state(null);

  const handleConfirm = async () => {
    if (!pendingConfirm) return;
    isPending = true;
    try {
      const res = await fetch('/api/memories/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          user_name: userName,
          conflicting_id: pendingConfirm.conflictingId,
          pending_fact: pendingConfirm.pendingFact,
          conflict_type: pendingConfirm.conflictType
        })
      });

      if (!res.ok) {
        throw new Error("일정 변경 처리에 실패했습니다.");
      }

      chatLogs = [...chatLogs, { sender: 'bot', text: '✔️ 일정을 정상적으로 업데이트하였습니다.', timestamp: new Date() }];
      
      if (onConfirmSuccess) {
        onConfirmSuccess();
      }
    } catch (err) {
      chatLogs = [...chatLogs, { sender: 'system', text: `오류: ${err.message}`, timestamp: new Date() }];
    } finally {
      pendingConfirm = null;
      isPending = false;
    }
  };

  const handleCancel = () => {
    chatLogs = [...chatLogs, { sender: 'bot', text: '❌ 예약을 취소하였습니다. 기존 일정이 보존됩니다.', timestamp: new Date() }];
    pendingConfirm = null;
  };

  // 채팅 전송 비동기 연동 함수
  const sendMessage = async (e) => {
    e.preventDefault();
    if (!inputVal.trim() || isPending) return;

    const userText = inputVal;
    inputVal = ""; // 입력창 즉시 초기화

    // 1. 사용자 발화 추가
    chatLogs = [...chatLogs, { sender: 'user', text: userText, timestamp: new Date() }];
    isPending = true;

    try {
      const res = await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_name: userName, text: userText })
      });

      if (!res.ok) {
        let errMsg = "서버와의 통신에 실패했습니다.";
        try {
          const errData = await res.json();
          if (errData && errData.error) errMsg = errData.error;
        } catch (_) {}
        throw new Error(errMsg);
      }

      const data = await res.json();
      const rawResponse = data.final_response;
      activeFactContext = data.fact_context || "";
      traceSteps = data.trace_steps || [];

      // 2. 에이전트로부터 전달받은 분석용 JSON 디버그 정보 파싱 시도
      try {
        const parsed = JSON.parse(rawResponse);
        latestDebugInfo = parsed; // 디버그 뷰어 바인딩

        // 만약 UNKNOWN(일반 질문)이어서 response 필드에 자연어가 들어있다면 이를 챗봇에 출력
        if (parsed.intent === "UNKNOWN" && parsed.response) {
          const parsedHtml = marked.parse(parsed.response, { async: false });
          chatLogs = [...chatLogs, { sender: 'bot', text: parsedHtml, timestamp: new Date() }];
        } else {
          // 기기 제어(WRITE)나 조회(QUERY)인 경우 메인 에이전트가 뱉은 요약 텍스트를 출력
          chatLogs = [...chatLogs, { sender: 'bot', text: "기억 분석 및 데이터베이스 처리를 완료했습니다. 좌측 모니터를 확인해 보세요.", timestamp: new Date() }];
        }
      } catch {
        // 단순 텍스트나 전체 메인 에이전트의 완성형 평문 답변인 경우
        const parsedHtml = marked.parse(rawResponse, { async: false });
        chatLogs = [...chatLogs, { sender: 'bot', text: parsedHtml, timestamp: new Date() }];
      }

      if (data.confirm_required) {
        pendingConfirm = {
          conflictingId: data.conflicting_id,
          pendingFact: data.pending_fact,
          conflictType: data.conflict_type
        };
      } else {
        pendingConfirm = null;
      }

    } catch (err) {
      chatLogs = [...chatLogs, { sender: 'system', text: `오류: ${err.message}`, timestamp: new Date() }];
    } finally {
      isPending = false;
    }
  };
</script>

<div class="chat-container">
  <!-- 1. 대화 로그 내역 뷰어 -->
  <div class="chat-history">
    {#if chatLogs.length === 0}
      <div class="chat-empty">
        🤖 MOSS 스마트홈 서비스 비서입니다.<br/>
        "거실 온도를 22도로 해줘" 혹은 "내 역할이 뭔지 말해줘" 등 질문을 입력해 보세요.
      </div>
    {/if}
    {#each chatLogs as log}
      <div class="chat-bubble-wrapper {log.sender}">
        <div class="chat-bubble">
          {#if log.sender === 'bot'}
            {@html log.text}
          {:else}
            {log.text}
          {/if}
        </div>
      </div>
    {/each}
    {#if isPending}
      <div class="chat-bubble-wrapper bot loading-state">
        <div class="chat-bubble loading-dots">
          <span></span><span></span><span></span>
        </div>
      </div>
    {/if}

    <!-- 확인 요구 버튼 렌더링 -->
    {#if pendingConfirm}
      <div class="chat-bubble-wrapper bot confirm-box-wrapper">
        <div class="chat-bubble confirm-box">
          {#if pendingConfirm.conflictType === 'VACATION_OVERLAP'}
            <p class="confirm-msg">⚠️ 해당 시간에는 연차 휴가가 등록되어 있습니다. 연차 상태를 유지한 채 회의 일정을 추가 등록하시겠습니까?</p>
          {:else}
            <p class="confirm-msg">⚠️ 일정 충돌이 발생했습니다. 기존 일정을 취소하고 변경하시겠습니까?</p>
          {/if}
          <div class="confirm-actions">
            <button onclick={handleConfirm} class="confirm-btn yes-btn" disabled={isPending}>
              {#if pendingConfirm.conflictType === 'VACATION_OVERLAP'}
                휴가 중 회의 추가 등록
              {:else}
                기존 일정 취소 후 변경
              {/if}
            </button>
            <button onclick={handleCancel} class="confirm-btn no-btn" disabled={isPending}>
              등록 취소
            </button>
          </div>
        </div>
      </div>
    {/if}
  </div>

  <!-- 2. 사용자 입력 전송 폼 -->
  <form onsubmit={sendMessage} class="chat-input-form">
    <input
      type="text"
      bind:value={inputVal}
      placeholder={pendingConfirm ? "결정을 선택해주세요..." : "에이전트에게 지시할 내용을 입력하세요..."}
      class="chat-input"
      disabled={isPending || pendingConfirm !== null}
    />
    <button type="submit" class="chat-send-btn" disabled={isPending || pendingConfirm !== null || !inputVal.trim()}>
      전송
    </button>
  </form>

  <!-- 3. 하단 실시간 의도 파싱 디버거 패널 -->
  {#if latestDebugInfo || activeFactContext}
    <div class="debug-panel">
      <div class="debug-header">
        🔍 Real-time Agent Intent Analyzer & Context Debug
      </div>
      <div class="debug-body">
        {#if latestDebugInfo}
          <div class="debug-row">
            <span class="debug-label">Detected Intent:</span>
            <span class="debug-val intent-{latestDebugInfo.intent}">{latestDebugInfo.intent}</span>
          </div>
          <div class="debug-row">
            <span class="debug-label">Extracted Facts:</span>
            <pre class="debug-json">{JSON.stringify(latestDebugInfo.extracted_facts, null, 2)}</pre>
          </div>
        {/if}
        {#if activeFactContext}
          <div class="debug-row" style="flex-direction: column; align-items: stretch; margin-top: 8px;">
            <span class="debug-label" style="margin-bottom: 4px;">Synthesized Fact Context (Main Agent Input):</span>
            <pre class="debug-json" style="max-height: 180px; white-space: pre-wrap; word-break: break-all;">{activeFactContext}</pre>
          </div>
        {/if}
        {#if traceSteps && traceSteps.length > 0}
          <div class="debug-row" style="flex-direction: column; align-items: stretch; margin-top: 12px; border-top: 1px solid #334155; padding-top: 12px;">
            <span class="debug-label" style="margin-bottom: 8px; color: #38bdf8; font-weight: bold;">🤖 Agent Execution Pipeline Trace:</span>
            <div class="timeline-container">
              {#each traceSteps as step, i}
                <div class="timeline-step">
                  <div class="step-title">
                    <span class="step-num">Step {i+1}</span>
                    <strong class="step-name">{step.agent_name}</strong>
                  </div>
                  <div class="step-details">
                    <div class="io-box">
                      <span class="io-label">Input</span>
                      <pre class="io-content">{step.input}</pre>
                    </div>
                    <div class="io-box">
                      <span class="io-label">Output</span>
                      <pre class="io-content">{step.output}</pre>
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .chat-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    gap: 16px;
  }

  .chat-history {
    flex-grow: 1;
    background: #1e293b;
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 12px;
    padding: 16px;
    height: 300px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .chat-empty {
    text-align: center;
    color: #64748b;
    font-size: 14px;
    line-height: 1.6;
    margin: auto;
  }

  /* 말풍선 공통 스타일 */
  .chat-bubble-wrapper {
    display: flex;
    width: 100%;
  }

  .chat-bubble {
    max-width: 75%;
    padding: 10px 16px;
    border-radius: 14px;
    font-size: 14px;
    line-height: 1.5;
  }

  .chat-bubble :global(p) {
    margin: 0 0 8px 0;
  }
  .chat-bubble :global(p:last-child) {
    margin-bottom: 0;
  }
  .chat-bubble :global(ul), .chat-bubble :global(ol) {
    margin: 8px 0;
    padding-left: 20px;
  }
  .chat-bubble :global(li) {
    margin-bottom: 4px;
  }
  .chat-bubble :global(code) {
    background: rgba(0, 0, 0, 0.25);
    padding: 2px 4px;
    border-radius: 4px;
    font-family: monospace;
    font-size: 13px;
  }
  .chat-bubble :global(pre) {
    background: rgba(0, 0, 0, 0.4);
    padding: 10px;
    border-radius: 6px;
    overflow-x: auto;
    margin: 8px 0;
  }
  .chat-bubble :global(pre code) {
    background: none;
    padding: 0;
  }

  /* 사용자 말풍선 (우측 정렬) */
  .user {
    justify-content: flex-end;
  }
  .user .chat-bubble {
    background: #3b82f6; /* blue-500 */
    color: #ffffff;
    border-bottom-right-radius: 2px;
  }

  /* 비서 말풍선 (좌측 정렬) */
  .bot {
    justify-content: flex-start;
  }
  .bot .chat-bubble {
    background: #334155; /* slate-700 */
    color: #f1f5f9;
    border-bottom-left-radius: 2px;
  }

  /* 시스템 경고 말풍선 */
  .system {
    justify-content: center;
  }
  .system .chat-bubble {
    background: rgba(220, 38, 38, 0.1);
    border: 1px solid rgba(220, 38, 38, 0.2);
    color: #fca5a5;
    font-size: 12px;
  }

  /* 입력 폼 */
  .chat-input-form {
    display: flex;
    gap: 10px;
  }

  .chat-input {
    flex-grow: 1;
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 10px;
    color: #f8fafc;
    padding: 12px 16px;
    font-size: 14px;
    transition: all 0.2s ease;
  }

  .chat-input:focus {
    outline: none;
    border-color: #3b82f6;
  }

  .chat-send-btn {
    background: #3b82f6;
    color: #ffffff;
    border: none;
    border-radius: 10px;
    padding: 0 20px;
    font-weight: 600;
    cursor: pointer;
    transition: background 0.15s ease;
  }

  .chat-send-btn:hover:not(:disabled) {
    background: #2563eb;
  }

  .chat-send-btn:disabled {
    background: #1e293b;
    color: #475569;
    cursor: not-allowed;
    border: 1px solid #334155;
  }

  /* 로딩 애니메이션 */
  .loading-dots {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 12px 20px !important;
  }

  .loading-dots span {
    width: 6px;
    height: 6px;
    background: #94a3b8;
    border-radius: 50%;
    animation: bounce 1.4s infinite ease-in-out both;
  }

  .loading-dots span:nth-child(1) { animation-delay: -0.32s; }
  .loading-dots span:nth-child(2) { animation-delay: -0.16s; }

  @keyframes bounce {
    0%, 80%, 100% { transform: scale(0); }
    40% { transform: scale(1.0); }
  }

  /* 실시간 디버그 패널 */
  .debug-panel {
    background: rgba(15, 23, 42, 0.8);
    border: 1px solid #334155;
    border-radius: 10px;
    padding: 12px 16px;
    font-family: monospace;
    font-size: 12px;
  }

  .debug-header {
    font-weight: bold;
    color: #94a3b8;
    border-bottom: 1px solid #334155;
    padding-bottom: 6px;
    margin-bottom: 8px;
  }

  .debug-row {
    margin-bottom: 6px;
    display: flex;
    gap: 8px;
    align-items: flex-start;
  }

  .debug-label {
    color: #64748b;
  }

  .debug-val {
    font-weight: bold;
  }

  .intent-WRITE { color: #10b981; }
  .intent-QUERY { color: #38bdf8; }
  .intent-UNKNOWN { color: #f43f5e; }

  .debug-json {
    margin: 0;
    background: rgba(30, 41, 59, 0.4);
    padding: 6px 12px;
    border-radius: 6px;
    color: #cbd5e1;
    overflow-x: auto;
    max-height: 120px;
    width: 100%;
  }

  /* 타임라인 스타일 */
  .timeline-container {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .timeline-step {
    background: rgba(30, 41, 59, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    padding: 10px;
  }
  .step-title {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 6px;
  }
  .step-num {
    background: #3b82f6;
    color: #fff;
    font-size: 10px;
    font-weight: bold;
    padding: 2px 6px;
    border-radius: 12px;
  }
  .step-name {
    color: #e2e8f0;
    font-size: 13px;
  }
  .step-details {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .io-box {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .io-label {
    color: #64748b;
    font-size: 11px;
    font-weight: bold;
  }
  .io-content {
    margin: 0;
    background: rgba(15, 23, 42, 0.6);
    padding: 6px 10px;
    border-radius: 4px;
    color: #cbd5e1;
    font-family: monospace;
    font-size: 11px;
    white-space: pre-wrap;
    word-break: break-all;
  }

  /* 확인 팝업 스타일 */
  .confirm-box-wrapper {
    margin-top: 4px;
  }

  .confirm-box {
    background: #1e293b !important;
    border: 1px solid #3b82f6 !important;
    border-radius: 12px;
    padding: 14px 18px !important;
    max-width: 85% !important;
  }

  .confirm-msg {
    margin: 0 0 10px 0;
    color: #e2e8f0;
    font-weight: 600;
    font-size: 13px;
  }

  .confirm-actions {
    display: flex;
    gap: 8px;
  }

  .confirm-btn {
    border: none;
    border-radius: 8px;
    padding: 8px 14px;
    font-size: 12px;
    font-weight: 700;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .yes-btn {
    background: #3b82f6;
    color: #ffffff;
  }

  .yes-btn:hover:not(:disabled) {
    background: #2563eb;
  }

  .no-btn {
    background: #334155;
    color: #94a3b8;
    border: 1px solid #475569;
  }

  .no-btn:hover:not(:disabled) {
    background: #475569;
    color: #f1f5f9;
  }

  .confirm-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
