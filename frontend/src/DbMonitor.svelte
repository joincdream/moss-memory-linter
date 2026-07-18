<script>
  // Svelte 5 Props 수신 명세
  let { userName, refreshTrigger = 0 } = $props();

  // Svelte 5 반응형 상태 룬 선언
  let memories = $state([]);
  let errorMsg = $state("");

  // Svelte 5 파생 룬을 활용한 데이터 분류
  let activeMemories = $derived(memories.filter(m => m.status === 'active'));
  let inactiveMemories = $derived(memories.filter(m => m.status === 'inactive'));

  // 공간별 스마트홈 기기 상태 파생 데이터
  let smartHomeStatus = $derived(
    activeMemories
      .filter(m => m.domain === 'SmartHome' || m.domain === 'UserStatus')
      .reduce((acc, m) => {
        if (!acc[m.subject]) acc[m.subject] = {};
        acc[m.subject][m.predicate] = m.object_value;
        return acc;
      }, {})
  );

  // Svelte 5 $effect 룬을 통한 비동기 데이터 폴링 기동
  $effect(() => {
    // refreshTrigger가 변경되면 이 effect가 자동으로 재기동되어 데이터를 갱신합니다.
    const _ = refreshTrigger;

    const fetchMemories = async () => {
      try {
        const res = await fetch(`/api/memories?user_name=${encodeURIComponent(userName)}`);
        if (!res.ok) {
          throw new Error(`HTTP 에러 발생 (코드: ${res.status})`);
        }
        const data = await res.json();
        memories = data;
        errorMsg = "";
      } catch (err) {
        errorMsg = err.message;
      }
    };

    // 마운트 시 즉시 1회 기동
    fetchMemories();

    // 5초 간격 폴링
    const timer = setInterval(fetchMemories, 5000);

    // cleanup 함수를 리턴하여 언마운트 시 타이머 가비지 컬렉션
    return () => {
      clearInterval(timer);
    };
  });

  // 기억 레코드 영구 삭제 함수
  const deleteMemory = async (id) => {
    if (!confirm(`[경고] ID #${id} 기억 레코드를 데이터베이스에서 영구적으로 삭제하시겠습니까?\n이 작업은 되돌릴 수 없습니다.`)) {
      return;
    }

    try {
      const res = await fetch(`/api/memories/${id}`, {
        method: 'DELETE'
      });

      if (!res.ok) {
        const errorData = await res.json();
        throw new Error(errorData.error || "삭제에 실패했습니다.");
      }

      // 상태 업데이트를 통한 UI 동기화
      memories = memories.filter(m => m.id !== id);
    } catch (err) {
      alert(`삭제 오류: ${err.message}`);
    }
  };
</script>

<div class="monitor-container">
  {#if errorMsg}
    <div class="error-alert">
      ⚠️ 데이터 로드 오류: {errorMsg}
    </div>
  {/if}

  <!-- 상단 실시간 대시보드 요약 패널 -->
  <div class="dashboard-summary">
    <!-- 1. 스마트홈 기기 실시간 상태 보드 -->
    <div class="summary-section">
      <div class="summary-title">🏠 실시간 가전 기기 상태</div>
      <div class="device-cards">
        {#each Object.entries(smartHomeStatus) as [space, props]}
          <div class="device-card">
            <div class="card-space-name">
              {space === 'LivingRoom' ? '🛋️ 거실' : space === 'BedRoom' ? '🛏️ 안방' : space === 'Kitchen' ? '🍳 주방' : `🚪 ${space}`}
            </div>
            <div class="card-device-props">
              {#each Object.entries(props) as [key, val]}
                <div class="prop-item">
                  <span class="prop-key">{key === 'target_temp' ? '온도' : key === 'light_status' ? '전등' : key}</span>
                  <span class="prop-val {val === 'ON' ? 'light-on' : val === 'OFF' ? 'light-off' : ''}">
                    {val === 'ON' ? '켜짐' : val === 'OFF' ? '꺼짐' : `${val}°C`}
                  </span>
                </div>
              {/each}
            </div>
          </div>
        {:else}
          <div class="empty-summary">켜지거나 활성화된 가전 기기가 없습니다.</div>
        {/each}
      </div>
    </div>

    <!-- 2. 활성 일정 및 린팅 만료 이력 보드 -->
    <div class="summary-section">
      <div class="summary-title">📅 일정 및 기억 린팅(정합성) 상태</div>
      <div class="schedule-summary-list">
        <!-- 활성 일정 -->
        {#each activeMemories.filter(m => m.domain === 'Schedule' || m.subject === 'Calendar') as m}
          <div class="sched-summary-item active-sched">
            <span class="status-dot green"></span>
            <span class="sched-text"><strong>{m.subject}</strong>: {m.object_value}</span>
            {#if m.start_time}
              <span class="sched-date">{m.start_time.split(' ')[0]}</span>
            {/if}
          </div>
        {/each}

        <!-- 만료된 과거 기억 -->
        {#each inactiveMemories as m}
          <div class="sched-summary-item inactive-sched">
            <span class="status-dot red"></span>
            <span class="sched-text deleted">
              [{m.domain}] {m.subject} ({m.object_value})
            </span>
            <span class="linter-badge">Linter 만료처리됨</span>
          </div>
        {/each}

        {#if activeMemories.filter(m => m.domain === 'Schedule' || m.subject === 'Calendar').length === 0 && inactiveMemories.length === 0}
          <div class="empty-summary">등록된 일정 정보가 없습니다.</div>
        {/if}
      </div>
    </div>
  </div>

  <div class="table-title">💾 SQLite 기억 데이터베이스 원본 레코드 (memory 테이블)</div>
  <div class="table-wrapper">
    <table class="memory-table">
      <thead>
        <tr>
          <th style="width: 50px;">ID</th>
          <th style="width: 90px;">Domain</th>
          <th style="width: 100px;">Subject</th>
          <th style="width: 100px;">Predicate</th>
          <th>Object Value</th>
          <th style="width: 80px;">Status</th>
          <th>Reservation Time Range</th>
          <th style="width: 70px; text-align: center;">Action</th>
        </tr>
      </thead>
      <tbody>
        {#if memories.length === 0}
          <tr>
            <td colspan="8" class="empty-state">
              기억 데이터베이스가 비어 있습니다. 챗봇을 통해 가전 지시나 일정을 입력해 보세요.
            </td>
          </tr>
        {:else}
          {#each memories as memory (memory.id)}
            <tr class="memory-row {memory.status === 'inactive' ? 'row-inactive' : ''}">
              <td class="id-cell">#{memory.id}</td>
              <td class="domain-cell">{memory.domain}</td>
              <td class="subject-cell">{memory.subject}</td>
              <td class="predicate-cell">{memory.predicate}</td>
              <td class="val-cell">
                <span class="value-badge">{memory.object_value}</span>
              </td>
              <td>
                <span class="status-badge {memory.status === 'active' ? 'active-badge' : 'inactive-badge'}">
                  {memory.status}
                </span>
              </td>
              <td class="time-cell">
                {#if memory.start_time}
                  <span class="time-icon">📅</span> {memory.start_time}
                  {#if memory.end_time}
                    ~ {memory.end_time}
                  {/if}
                {:else}
                  <span class="no-time">N/A</span>
                {/if}
              </td>
              <td style="text-align: center; vertical-align: middle;">
                <button 
                  class="delete-btn" 
                  onclick={() => deleteMemory(memory.id)}
                  title="영구 삭제"
                >
                  🗑️
                </button>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</div>

<style>
  /* 대시보드 요약 스타일 */
  .dashboard-summary {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
    margin-bottom: 8px;
  }

  .summary-section {
    background: #0f172a;
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 12px;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .summary-title {
    font-size: 14px;
    font-weight: 700;
    color: #94a3b8;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
    padding-bottom: 8px;
  }

  .device-cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
    gap: 10px;
    overflow-y: auto;
    max-height: 150px;
  }

  .device-card {
    background: #1e293b;
    border: 1px solid rgba(255, 255, 255, 0.03);
    border-radius: 8px;
    padding: 10px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .card-space-name {
    font-size: 12px;
    font-weight: 700;
    color: #cbd5e1;
  }

  .card-device-props {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .prop-item {
    display: flex;
    justify-content: space-between;
    font-size: 11px;
  }

  .prop-key {
    color: #64748b;
  }

  .prop-val {
    font-weight: 700;
    color: #38bdf8;
  }

  .light-on {
    color: #fbbf24;
  }

  .light-off {
    color: #475569;
  }

  .schedule-summary-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    overflow-y: auto;
    max-height: 150px;
  }

  .sched-summary-item {
    display: flex;
    align-items: center;
    gap: 8px;
    background: #1e293b;
    padding: 8px 12px;
    border-radius: 6px;
    font-size: 12px;
  }

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    display: inline-block;
  }

  .status-dot.green { background: #10b981; }
  .status-dot.red { background: #ef4444; }

  .sched-text {
    flex-grow: 1;
    color: #e2e8f0;
  }

  .sched-text.deleted {
    color: #64748b;
    text-decoration: line-through;
  }

  .sched-date {
    font-size: 11px;
    color: #475569;
  }

  .linter-badge {
    font-size: 10px;
    background: rgba(239, 68, 68, 0.1);
    color: #f87171;
    border: 1px solid rgba(239, 68, 68, 0.2);
    padding: 1px 6px;
    border-radius: 4px;
    font-weight: bold;
  }

  .table-title {
    font-size: 13px;
    font-weight: 700;
    color: #64748b;
    margin-top: 8px;
    margin-bottom: 4px;
  }

  .empty-summary {
    text-align: center;
    color: #475569;
    font-size: 12px;
    padding: 24px 0;
    font-style: italic;
  }

  .monitor-container {
    display: flex;
    flex-direction: column;
    gap: 12px;
    height: 100%;
  }

  .error-alert {
    background: #450a0a;
    border: 1px solid #dc2626;
    color: #fca5a5;
    padding: 10px 16px;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 500;
  }

  .table-wrapper {
    overflow-x: auto;
    border-radius: 12px;
    border: 1px solid rgba(255, 255, 255, 0.05);
    background: #1e293b; /* slate-800 */
    max-height: 520px;
    overflow-y: auto;
  }

  .memory-table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
    font-size: 14px;
  }

  .memory-table th {
    background: rgba(15, 23, 42, 0.8);
    color: #94a3b8;
    padding: 12px 16px;
    font-weight: 600;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
    position: sticky;
    top: 0;
    z-index: 10;
  }

  .memory-table td {
    padding: 12px 16px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.02);
    color: #cbd5e1;
    font-weight: 500;
  }

  /* 테이블 행 스타일링 */
  .memory-row {
    transition: background 0.15s ease;
  }

  .memory-row:hover {
    background: rgba(255, 255, 255, 0.02);
  }

  /* 만료된 데이터는 시각적으로 어둡고 빗금선 효과 부여 */
  .row-inactive {
    background: rgba(15, 23, 42, 0.3) !important;
    opacity: 0.5;
  }

  .row-inactive td {
    text-decoration: line-through;
    text-decoration-color: rgba(244, 63, 94, 0.3);
  }

  .id-cell {
    color: #64748b !important;
    font-family: monospace;
    font-weight: 700 !important;
  }

  .domain-cell, .subject-cell, .predicate-cell {
    font-family: monospace;
    color: #e2e8f0;
  }

  .value-badge {
    background: rgba(56, 189, 248, 0.1);
    color: #38bdf8;
    padding: 2px 8px;
    border-radius: 6px;
    font-weight: 600;
    font-size: 13px;
  }

  /* 활성/만료 상태 뱃지 */
  .status-badge {
    padding: 2px 8px;
    border-radius: 12px;
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    display: inline-block;
  }

  .active-badge {
    background: rgba(16, 185, 129, 0.15);
    color: #10b981;
    border: 1px solid rgba(16, 185, 129, 0.2);
  }

  .inactive-badge {
    background: rgba(244, 63, 94, 0.15);
    color: #f43f5e;
    border: 1px solid rgba(244, 63, 94, 0.2);
  }

  .time-cell {
    font-size: 13px;
    color: #cbd5e1;
  }

  .time-icon {
    margin-right: 4px;
  }

  .no-time {
    color: #475569;
    font-style: italic;
  }

  .empty-state {
    text-align: center;
    color: #64748b;
    padding: 48px 16px !important;
    font-style: italic;
  }

  .delete-btn {
    background: none;
    border: none;
    cursor: pointer;
    font-size: 14px;
    padding: 6px;
    border-radius: 6px;
    transition: all 0.2s ease;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .delete-btn:hover {
    background: rgba(244, 63, 94, 0.15);
    transform: scale(1.1);
  }
</style>
