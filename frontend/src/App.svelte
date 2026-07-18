<script>
  import DbMonitor from './DbMonitor.svelte';
  import ChatBox from './ChatBox.svelte';

  // Svelte 5 룬을 활용한 사용자 식별자 세션 관리 및 리프레시 토글
  let userName = $state("yundream");
  let refreshTrigger = $state(0);

  const triggerRefresh = () => {
    refreshTrigger += 1;
  };
</script>

<main class="dashboard-container">
  <header class="dashboard-header">
    <div class="logo-area">
      <span class="logo-icon">🤖</span>
      <h1>MOSS Memory Control Dashboard</h1>
    </div>
    <div class="user-session">
      <span class="session-label">User Session:</span>
      <input type="text" bind:value={userName} class="session-input" />
    </div>
  </header>

  <div class="dashboard-grid">
    <!-- 좌측 영역: 실시간 데이터베이스 모니터 -->
    <section class="panel-section monitor-panel">
      <div class="panel-header">
        <span class="panel-title-icon">📊</span>
        <h2>SQLite Real-Time Memory Monitor</h2>
      </div>
      <div class="panel-body">
        <DbMonitor {userName} {refreshTrigger} />
      </div>
    </section>

    <!-- 우측 영역: MOSS 챗봇 대화방 및 디버거 -->
    <section class="panel-section chat-panel">
      <div class="panel-header">
        <span class="panel-title-icon">💬</span>
        <h2>MOSS Conversational Agent</h2>
      </div>
      <div class="panel-body">
        <ChatBox {userName} onConfirmSuccess={triggerRefresh} />
      </div>
    </section>
  </div>
</main>

<style>
  /* 프리미엄 다크 테마 CSS 디자인 시스템 */
  .dashboard-container {
    max-width: 95%;
    margin: 0 auto;
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 20px;
    min-height: 90vh;
  }

  .dashboard-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 24px;
    background: rgba(30, 41, 59, 0.7); /* slate-800 with glassmorphism */
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 16px;
    backdrop-filter: blur(12px);
  }

  .logo-area {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .logo-icon {
    font-size: 28px;
  }

  .dashboard-header h1 {
    margin: 0;
    font-family: 'Outfit', sans-serif;
    font-weight: 800;
    font-size: 22px;
    letter-spacing: -0.5px;
    background: linear-gradient(135deg, #38bdf8, #818cf8);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .user-session {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .session-label {
    font-size: 14px;
    color: #94a3b8; /* slate-400 */
    font-weight: 500;
  }

  .session-input {
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 8px;
    color: #f8fafc;
    padding: 6px 12px;
    font-size: 14px;
    font-weight: 600;
    width: 120px;
    transition: all 0.2s ease;
  }

  .session-input:focus {
    outline: none;
    border-color: #38bdf8;
    box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.2);
  }

  /* 메인 그리드 레이아웃 */
  .dashboard-grid {
    display: grid;
    grid-template-columns: 1.2fr 1fr; /* 모니터링 테이블을 더 넓게 배치 */
    gap: 20px;
    flex-grow: 1;
  }

  .panel-section {
    background: rgba(30, 41, 59, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 20px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    backdrop-filter: blur(16px);
  }

  .panel-header {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 16px 20px;
    background: rgba(15, 23, 42, 0.6);
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  }

  .panel-title-icon {
    font-size: 20px;
  }

  .panel-header h2 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: #f1f5f9;
  }

  .panel-body {
    padding: 20px;
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    min-height: 500px;
  }

  @media (max-width: 1024px) {
    .dashboard-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
