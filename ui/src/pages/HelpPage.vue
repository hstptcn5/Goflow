<script setup>
function openNodeDocs() {
  window.open('/NODES.md', '_blank', 'noopener,noreferrer');
}
</script>

<template>
  <div class="page-stack">
    <section class="page-toolbar">
      <div>
        <h2>Trợ giúp</h2>
        <p>Tài liệu vận hành nhanh để tạo, chạy và xử lý sự cố workflow.</p>
      </div>
    </section>

    <section class="workflow-grid">
      <article class="workflow-card">
        <div class="card-main">
          <h3>Hướng dẫn node</h3>
          <p>Mở tài liệu node đi kèm để xem tham số, credential và ví dụ cấu hình.</p>
        </div>
        <button class="btn btn-primary" type="button" @click="openNodeDocs">Mở tài liệu</button>
      </article>
      <article class="workflow-card">
        <div class="card-main">
          <h3>Checklist workflow đầu tiên</h3>
          <p>Thêm trigger → thêm action → cấu hình trường bắt buộc → nối node → chạy thử → kiểm tra output → sau cùng mới bật workflow.</p>
        </div>
      </article>
      <article class="workflow-card">
        <div class="card-main">
          <h3>Xử lý sự cố</h3>
          <p>Nếu API không truy cập được, kiểm tra tiến trình Goflow và khóa API trong Cài đặt. Nếu live update mất kết nối, kết quả chạy workflow vẫn lấy REST API làm nguồn dữ liệu chính xác.</p>
        </div>
      </article>
    </section>
  </div>
</template>
