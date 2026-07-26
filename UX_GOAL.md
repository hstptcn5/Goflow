# UX GOAL: Hoàn thiện Goflow thành workflow editor dễ học, dễ dùng và dễ debug

## 1. Mục tiêu của tài liệu

Tài liệu này là North Star specification cho giai đoạn cải thiện trải nghiệm người dùng của Goflow.

`GOAL.md` trước đó tập trung vào:

* Workflow runtime.
* CLI.
* MCP.
* Security.
* Concurrency.
* Idempotency.
* Reliability.

`UX_GOAL.md` tập trung vào một vấn đề khác:

> Người dùng có thể tạo, cấu hình, chạy và sửa lỗi workflow một cách trực quan mà không cần hiểu kiến trúc bên trong Goflow.

Không thay thế hoặc làm suy yếu các yêu cầu trong `GOAL.md`.

Runtime hiện tại phải tiếp tục giữ các nguyên tắc:

* Single Go binary.
* Embedded frontend.
* SQLite mặc định.
* Không bắt buộc Docker.
* Không bắt buộc Node.js ở production.
* Một execution engine duy nhất.
* UI sử dụng REST API và WebSocket hiện có.
* Không tạo execution path riêng cho frontend.
* Không làm giảm security, redaction, authorization hoặc concurrency protection.

---

# 2. Vấn đề sản phẩm cần giải quyết

Goflow đã có workflow engine, CLI, MCP, credential vault, execution history và nhiều loại node.

Tuy nhiên, người dùng mới có thể gặp các vấn đề:

* Không biết nên bắt đầu từ đâu.
* Không tìm thấy node cần dùng nhanh chóng.
* Không hiểu cách kết nối dữ liệu giữa các node.
* Không biết field nào bắt buộc.
* Không biết workflow đã được lưu hay chưa.
* Không biết node nào đang chạy hoặc thất bại.
* Không thấy rõ input và output của từng node.
* Không biết tại sao một node lỗi.
* Không biết nên sửa gì sau khi gặp lỗi.
* Giao diện có quá nhiều nút và thông tin cùng lúc.
* Người từng dùng n8n hoặc công cụ tương tự vẫn phải học lại luồng thao tác.

Backend chạy nhanh và ổn định là cần thiết, nhưng không đủ để người dùng tiếp tục sử dụng sản phẩm.

UX GOAL phải chuyển Goflow từ:

> Một workflow engine có visual editor.

thành:

> Một workflow product mà người dùng có thể học, xây dựng và debug workflow bằng chính giao diện của nó.

---

# 3. North Star

North Star của giai đoạn này:

> Một người chưa từng sử dụng Goflow có thể tạo và chạy workflow hữu ích đầu tiên trong dưới 10 phút, đồng thời xác định nguyên nhân một node thất bại trong dưới 2 phút.

Goflow phải tạo cảm giác quen thuộc với người đã dùng các workflow automation tool phổ biến, nhưng không sao chép giao diện, code, assets hoặc thương hiệu của sản phẩm khác.

Giao diện phải ưu tiên:

```text
Hiểu được
→ cấu hình được
→ chạy được
→ debug được
→ tin tưởng để dùng lại
```

Không ưu tiên trang trí hoặc hiệu ứng trước khả năng sử dụng.

---

# 4. Người dùng mục tiêu

## 4.1. Developer và technical user

Họ cần:

* Tự host nhanh.
* Tạo workflow tích hợp API.
* Chạy script và job nội bộ.
* Debug dữ liệu JSON.
* Quản lý credential.
* Kết hợp UI với CLI và MCP.
* Xem execution rõ ràng.
* Không cần một hệ thống enterprise lớn.

## 4.2. Người từng sử dụng n8n hoặc công cụ workflow khác

Họ kỳ vọng:

* Có canvas.
* Có node picker.
* Có configuration panel.
* Có input/output inspector.
* Có execution history.
* Biết node nào chạy thành công hoặc thất bại.
* Có cách map dữ liệu từ node trước sang node sau.
* Có thể retry hoặc replay khi lỗi.

## 4.3. Người dùng kỹ thuật vừa phải

Họ có thể hiểu API, JSON và credential nhưng không muốn đọc source code hoặc tài liệu dài để hoàn thành workflow đầu tiên.

---

# 5. Chỉ số thành công

## 5.1. Time to first workflow

Một người dùng mới phải có thể:

1. Mở Goflow.
2. Chọn template hoặc tạo workflow.
3. Thêm và cấu hình các node.
4. Chạy workflow.
5. Xem kết quả.

Trong dưới 10 phút.

## 5.2. Node discovery

Người dùng phải tìm và thêm một node mong muốn trong dưới 15 giây.

## 5.3. Configuration clarity

Người dùng phải nhận biết ngay:

* Field nào bắt buộc.
* Credential nào cần chọn.
* Field nào có lỗi.
* Node đã cấu hình đủ hay chưa.

## 5.4. Debugging

Khi một node thất bại, người dùng phải xác định được:

* Node nào lỗi.
* Input thực tế của node.
* Parameter sau khi resolve.
* Error message.
* Số lần thử.
* Thời gian chạy.
* Output hoặc response nhận được.

Trong dưới 2 phút.

## 5.5. Usability testing

Ít nhất 4 trong 5 tester mục tiêu phải hoàn thành workflow ba node mà không cần người phát triển Goflow hướng dẫn trực tiếp.

## 5.6. Familiarity

Người từng dùng n8n hoặc công cụ tương tự phải hiểu được các hành động chính mà không cần đọc tài liệu trước:

* Add node.
* Connect node.
* Configure node.
* Run workflow.
* Inspect output.
* Retry failure.

---

# 6. Nguyên tắc thiết kế bắt buộc

## 6.1. Familiar, not copied

Học mental model từ các workflow tool đã phổ biến:

* Canvas trung tâm.
* Node picker có tìm kiếm.
* Click node để cấu hình.
* Input và output rõ ràng.
* Execution status hiển thị trên canvas.
* Quick-add node.
* History và replay.

Không sao chép:

* Source code.
* CSS.
* Logo.
* Icon độc quyền.
* Nội dung.
* Assets.
* Layout pixel-by-pixel.
* Thương hiệu của sản phẩm khác.

## 6.2. Progressive disclosure

Không hiển thị tất cả tùy chọn cùng lúc.

Mặc định chỉ hiển thị:

* Các field bắt buộc.
* Các field thường dùng.
* Credential.
* Resource.
* Operation.

Các cấu hình hiếm dùng phải nằm trong:

```text
Advanced options
```

## 6.3. One clear primary action

Mỗi màn hình chỉ nên có một hành động chính nổi bật.

Ví dụ trong editor:

```text
Test Workflow
```

hoặc:

```text
Save
```

Không để nhiều nút có cùng độ ưu tiên thị giác.

## 6.4. Explain errors where they happen

Không chỉ hiển thị lỗi bằng alert hoặc toast chung.

Lỗi phải xuất hiện tại:

* Node lỗi.
* Field cấu hình lỗi.
* Execution đang xem.
* Credential liên quan.
* Output panel.

## 6.5. No invisible state

Người dùng phải nhìn thấy:

* Workflow đã lưu hay chưa.
* Workflow active hay inactive.
* Execution đang chạy hay đã xong.
* Node chưa chạy, đang chạy, thành công, lỗi hoặc skipped.
* Data đang xem thuộc execution nào.
* Data đang xem là live hay từ lịch sử.

## 6.6. Backend remains source of truth

Frontend không tự giả định execution thành công.

Mọi trạng thái phải dựa trên:

* REST API.
* WebSocket events.
* Execution records.
* Workflow definitions.
* Node definitions.

---

# 7. Information architecture mới

## 7.1. App shell

Thay cấu trúc navbar quá tải bằng hai lớp rõ ràng.

### Navigation rail

```text
Workflows
Executions
Credentials
Templates
Nodes
Settings
Help
```

Navigation rail có thể collapse.

Không dùng modal làm cách điều hướng chính cho Workflows, Executions hoặc Credentials.

### Workflow top bar

Khi đang mở workflow, top bar hiển thị:

```text
Back
Workflow name
Saved / Unsaved
Active toggle
Test Workflow
Save
More actions
```

Menu `More actions` chứa:

```text
Export
Duplicate
Delete
Workflow settings
Interface settings
```

### Yêu cầu

* Bỏ nhãn `MVP` khỏi giao diện chính.
* Chỉ một primary button.
* Hiển thị rõ trạng thái `Saving`, `Saved`, `Unsaved`.
* Có cảnh báo khi rời workflow còn thay đổi chưa lưu.
* Không dùng inline style cho các thành phần chính.
* Navigation phải hoạt động với keyboard.

---

# 8. Onboarding

## 8.1. First-run experience

Khi chưa có workflow, hiển thị:

```text
Create from template
Create blank workflow
Import workflow
```

Template phải được ưu tiên vì giúp người dùng học bằng ví dụ chạy được.

## 8.2. First workflow checklist

Checklist hiển thị trong lần đầu:

```text
1. Add a trigger
2. Add an action
3. Configure required fields
4. Connect nodes
5. Test workflow
6. Inspect output
7. Activate workflow
```

Checklist phải tự cập nhật theo trạng thái thực.

## 8.3. Templates

Cần tối thiểu các template chạy được hoặc chỉ cần chỉnh ít thông tin:

* Manual Trigger → HTTP Request.
* Webhook → JSON Transform → Discord hoặc Telegram.
* Cron → HTTP Request.
* Manual Trigger → IF → two branches.
* Manual Trigger → JavaScript → JSON output.
* Sub-workflow example.

Mỗi template phải có:

* Mục đích.
* Input mong đợi.
* Credential cần dùng.
* Kết quả đầu ra.
* Các placeholder cần thay.
* Nút tạo workflow.

## 8.4. Inline guidance

Không bắt người dùng mở README để hiểu các bước cơ bản.

Node và field cần có:

* Mô tả ngắn.
* Ví dụ.
* Placeholder hữu ích.
* Link mở tài liệu chi tiết khi cần.

---

# 9. Node picker

## 9.1. Cách mở

Node picker phải mở được từ:

* Nút `Add step`.
* Dấu `+` sau một node.
* Dấu `+` trên edge.
* Double-click vào canvas.
* Keyboard shortcut.
* Command palette.

## 9.2. Nội dung picker

Picker phải có:

```text
Search
Recent
Favorites
Triggers
Actions
Logic
AI
Databases
Communication
Developer Tools
```

## 9.3. Mỗi node item

Hiển thị:

* Icon.
* Tên thân thiện.
* Mô tả một dòng.
* Category.
* Trigger hoặc action.
* Credential cần thiết.
* Trạng thái experimental khi phù hợp.

Không lấy internal node type làm tên chính.

Ví dụ:

```text
HTTP Request
Send an HTTP request and return the response.
```

Thay vì chỉ hiển thị:

```text
httpRequest
```

## 9.4. Hành vi

* Search không phân biệt chữ hoa/chữ thường.
* Search theo tên, type, category và description.
* Keyboard có thể chọn kết quả.
* Enter thêm node.
* Escape đóng picker.
* Node được thêm ở vị trí hợp lý.
* Quick-add tự nối node mới với node nguồn.
* Recent nodes cập nhật theo hành vi sử dụng.
* Favorites được lưu local.

## 9.5. Palette cũ

Node palette cố định có thể:

* Bị loại bỏ.
* Hoặc chuyển thành panel collapse.
* Hoặc chỉ mở khi người dùng yêu cầu.

Không được luôn chiếm diện tích canvas trên màn hình nhỏ.

---

# 10. Canvas

## 10.1. Visual hierarchy

Canvas phải tập trung vào workflow, không phải hiệu ứng.

Yêu cầu:

* Edge idle không animated.
* Edge mỏng và trung tính.
* Chỉ edge đang chạy được animate.
* Edge thành công, lỗi hoặc skipped có trạng thái riêng.
* Background nhẹ, không gây nhiễu.
* Node được căn và phân bố dễ đọc.

## 10.2. Layout direction

Mặc định ưu tiên:

```text
Left → Right
```

vì phù hợp với cách đọc input → process → output.

Nếu giữ Top → Bottom, cần chứng minh qua user test rằng hướng này dễ dùng hơn.

## 10.3. Node card

Node card phải hiển thị:

```text
Icon
Node name
Operation summary
Configuration state
Execution state
Duration hoặc attempts khi có
```

Ví dụ:

```text
HTTP Request
GET api.example.com/users
Credential: Internal API
Configured
```

Khi execution:

```text
Running
Success · 240 ms
Failed · 3 attempts
Skipped
```

## 10.4. Handles

* Input và output handle dễ nhận biết.
* IF node có nhãn `true` và `false`.
* Handle đủ lớn để click.
* Có quick-add button trên output.
* Không cần kéo dây chính xác vào điểm quá nhỏ.
* Edge connection có preview.

## 10.5. Canvas commands

Phải có:

* Undo.
* Redo.
* Copy.
* Paste.
* Duplicate node.
* Delete.
* Multi-select.
* Select all.
* Fit view.
* Zoom.
* Auto-layout.
* Align.
* Distribute.
* Keyboard navigation.

## 10.6. Validation trên canvas

Node chưa cấu hình đủ phải có badge:

```text
Missing 2 required fields
```

Workflow có lỗi cấu trúc phải chỉ rõ:

* Node ID trùng.
* Edge invalid.
* Cycle.
* Missing credential.
* Unknown node.
* Missing required parameter.

Không chờ đến khi chạy mới báo các lỗi có thể phát hiện trước.

---

# 11. Node configuration inspector

## 11.1. Cấu trúc panel

Panel node phải rộng đủ để đọc form và dữ liệu.

Tabs:

```text
Parameters
Input
Output
Logs
```

Có thể thêm tab `Help` nhưng không được làm tab chính quá nhiều.

## 11.2. Parameters tab

Thứ tự ưu tiên:

```text
Credential
Resource
Operation
Required parameters
Common optional parameters
Advanced options
```

Yêu cầu:

* Required field có dấu hiệu rõ ràng.
* Validation inline.
* Không chỉ báo lỗi khi save.
* Credential có nút tạo mới.
* Credential thiếu phải có hướng dẫn.
* Select option phải có label thân thiện.
* JSON field có editor.
* JavaScript field có code editor.
* Secret field không hiển thị plaintext.
* Node name có thể đổi nhưng không chiếm phần lớn panel.

## 11.3. Advanced options

Các field ít dùng phải nằm trong section collapse:

```text
Advanced options
```

Trạng thái collapse được nhớ theo node type.

## 11.4. Help

Node help phải nằm gần field liên quan.

Không dồn toàn bộ hướng dẫn vào một khối dài cuối panel.

Mỗi field có thể có:

* Tooltip.
* Description.
* Example.
* Link đến node guide.

---

# 12. Data mapping và expression

## 12.1. Input data viewer

Input tab hiển thị dữ liệu từ các node trước theo:

* JSON tree.
* Table khi dữ liệu dạng array.
* Raw view.
* Search.
* Copy value.
* Copy JSON path.

## 12.2. Mapping

Người dùng phải có thể:

* Click giá trị để chèn vào parameter.
* Kéo giá trị vào field.
* Chọn node nguồn.
* Chọn path từ data picker.
* Xem expression được tạo.

## 12.3. Expression mode

Mỗi field hỗ trợ expression cần có hai mode:

```text
Fixed
Expression
```

Expression editor phải hiển thị:

* Expression source.
* Preview resolved value.
* Error nếu resolve thất bại.
* Node/path đang tham chiếu.
* Autocomplete cho outputs đã biết.

## 12.4. Resolved preview

Trước khi chạy node, người dùng phải xem được parameter sau khi resolve bằng sample data hoặc execution data đã chọn.

Ví dụ:

```text
Expression:
{{$json.user.email}}

Resolved:
dev@example.com
```

## 12.5. Pin sample data

Cho phép pin output mẫu của một node để cấu hình các node sau mà không cần chạy toàn bộ workflow liên tục.

Pinned data phải được phân biệt rõ với live execution data.

---

# 13. Execution debugger

## 13.1. Execution selector

Editor phải có execution selector:

```text
Latest
Running
Failed
Specific execution
Pinned test data
```

Khi chọn execution, toàn bộ canvas và inspector phải hiển thị dữ liệu của execution đó.

## 13.2. Node status

Trạng thái chuẩn:

```text
NOT_RUN
QUEUED
RUNNING
SUCCESS
FAILED
CANCELLED
SKIPPED
```

Mỗi trạng thái có:

* Icon.
* Label.
* Màu.
* Tooltip.
* Accessibility text.

Không chỉ phân biệt bằng màu.

## 13.3. Node debug data

Khi click node trong execution, hiển thị:

```text
Input
Resolved parameters
Output
Error
Attempts
Duration
Start time
Finish time
Branch
Logs
```

Tất cả dữ liệu phải qua redaction phía backend.

## 13.4. Debug actions

Mục tiêu cuối cùng phải hỗ trợ:

```text
Run node
Run from here
Run until here
Retry failed node
Retry workflow
Replay execution
Replay with edited input
Cancel execution
Copy debug bundle
```

Nếu engine chưa hỗ trợ node-level execution an toàn, cần chia thành các milestone:

### Milestone đầu

* Retry full workflow.
* Replay full workflow.
* Copy debug bundle.
* Execution-specific inspector.

### Milestone tiếp theo

* Run node.
* Run from here.
* Run until here.
* Retry failed node.

Không giả lập node-level execution chỉ ở frontend.

## 13.5. Failed path

Khi workflow thất bại:

* Node lỗi được focus.
* Path dẫn đến node lỗi được highlight.
* Node chưa chạy được đánh dấu.
* Nhánh skipped được hiển thị nhẹ.
* Error panel tự mở.
* Có hành động tiếp theo rõ ràng.

Ví dụ:

```text
Authentication failed.
Check the selected credential or reconnect OAuth.
```

Thay vì chỉ:

```text
401 Unauthorized
```

## 13.6. Debug bundle

Debug bundle an toàn phải gồm:

* Workflow ID.
* Execution ID.
* Node ID.
* Node type.
* Status.
* Duration.
* Attempts.
* Redacted input.
* Redacted resolved parameters.
* Redacted output.
* Error.
* App version.
* OS/runtime information cần thiết.

Không chứa:

* Credential plaintext.
* Access token.
* Password.
* Cookie.
* Authorization header.
* Private key.

---

# 14. Execution history

Execution screen phải hỗ trợ:

* Filter theo workflow.
* Filter theo status.
* Filter theo trigger source.
* Search theo execution ID.
* Sort theo thời gian.
* Xem duration.
* Xem principal.
* Xem error summary.
* Open in editor.
* Retry.
* Replay.
* Cancel khi đang chạy.

Mỗi execution row phải hiển thị nhanh:

```text
Status
Workflow
Trigger source
Started time
Duration
Error summary
```

Không bắt người dùng mở từng record để biết execution nào thất bại.

---

# 15. Workflow management

Workflow list phải trở thành một page chính thay vì chỉ modal.

Mỗi workflow hiển thị:

* Name.
* Description.
* Active status.
* Last execution.
* Last execution status.
* Trigger types.
* Updated time.
* CLI exposure.
* MCP exposure.

Actions:

```text
Open
Run
Activate / Deactivate
Duplicate
Export
Delete
```

Có:

* Search.
* Filter active/inactive.
* Filter failed recently.
* Sort updated time.
* Empty state.
* Template creation.

---

# 16. Credential experience

Credential management phải:

* Phân loại credential.
* Cho biết node nào đang sử dụng credential.
* Hiển thị trạng thái OAuth connected/disconnected.
* Có test connection khi khả thi.
* Không hiển thị secret sau khi lưu.
* Cho phép replace secret.
* Hiển thị lỗi OAuth dễ hiểu.
* Có hướng dẫn setup theo từng loại credential.

Khi node cần credential nhưng chưa có:

```text
No compatible credential found
[Create credential]
```

Không chỉ hiển thị select rỗng.

---

# 17. Design system

## 17.1. Mục tiêu

Không chỉnh CSS theo từng component một cách tùy ý.

Phải xây design system tối thiểu gồm:

* Color tokens.
* Typography.
* Spacing.
* Border radius.
* Shadows.
* Icon sizes.
* Button hierarchy.
* Form controls.
* Tabs.
* Dialogs.
* Badges.
* Status colors.
* Empty states.
* Loading states.

## 17.2. Visual direction

Goflow nên:

* Gọn.
* Kỹ thuật.
* Tin cậy.
* Ít trang trí.
* Có khoảng trắng.
* Ưu tiên dữ liệu.
* Có contrast rõ.
* Không dùng quá nhiều màu cùng lúc.
* Không animate các thành phần không mang ý nghĩa.

## 17.3. Theme

Ít nhất hỗ trợ:

* Light theme hoàn chỉnh.

Dark theme có thể làm sau nếu không ảnh hưởng P0.

## 17.4. Icon

Sử dụng một icon system nhất quán.

Không trộn:

* Emoji.
* Text placeholder.
* SVG khác phong cách.
* Internal node type làm icon.

---

# 18. Accessibility

Phải hỗ trợ tối thiểu:

* Keyboard navigation.
* Visible focus state.
* Accessible labels.
* Không chỉ dùng màu để biểu thị status.
* Contrast phù hợp.
* Buttons có tên rõ.
* Dialog focus trap.
* Escape đóng dialog.
* Screen-reader status cho execution.
* Form error liên kết với field.
* Canvas commands có shortcut và menu tương đương.

---

# 19. Responsive behavior

Goflow chủ yếu dành cho desktop nhưng phải hoạt động hợp lý ở:

```text
1366 × 768
1440 × 900
1920 × 1080
```

Yêu cầu:

* Không che canvas bằng panel cố định không đóng được.
* Node picker và inspector có thể resize hoặc collapse.
* Top bar không overflow.
* Navigation rail collapse.
* Inspector không vượt chiều cao màn hình.
* Màn hình nhỏ hiển thị cảnh báo nếu editor không đủ không gian.

Mobile không phải mục tiêu editor chính.

Có thể hỗ trợ mobile cho:

* Xem execution.
* Xem workflow status.
* Activate/deactivate.
* Cancel execution.

---

# 20. Performance

UX không được đánh đổi bằng frontend nặng hoặc phản hồi chậm.

Mục tiêu:

* App shell hiển thị nhanh.
* Canvas pan/zoom mượt với workflow thông thường.
* Search node phản hồi ngay.
* Không rerender toàn canvas khi sửa một field.
* Không poll liên tục khi có WebSocket.
* Large JSON viewer phải virtualize hoặc collapse.
* Workflow với 100 node vẫn có thể thao tác.
* Không block UI khi parse execution logs lớn.

Cần benchmark tối thiểu:

```text
10 nodes
50 nodes
100 nodes
Large output payload
Long execution history
```

---

# 21. Công nghệ frontend và FlowGram decision gate

## 21.1. Mặc định

Tiếp tục sử dụng Vue 3 và Vue Flow trong giai đoạn cải thiện đầu tiên.

Không rewrite toàn bộ frontend ngay.

## 21.2. Prototype bắt buộc trước migration

Nếu cân nhắc FlowGram hoặc stack khác, phải tạo prototype độc lập đáp ứng:

* Load workflow hiện tại.
* Render nodes và edges.
* Add node.
* Edit node.
* Connect nodes.
* IF branch handles.
* Save đúng JSON hiện tại.
* Render execution status.
* Input/output inspector.
* Embed được trong single Go binary.
* Frontend build ổn định.
* License phù hợp.

## 21.3. Scorecard

So sánh Vue Flow cải tiến và FlowGram prototype theo:

| Tiêu chí                 | Trọng số |
| ------------------------ | -------: |
| User experience          |      25% |
| Data model compatibility |      15% |
| Debugger capability      |      15% |
| Data mapping capability  |      10% |
| Maintainability          |      10% |
| Bundle/runtime cost      |      10% |
| Testing capability       |      10% |
| Migration effort         |       5% |

Chỉ migrate khi FlowGram có lợi thế rõ ràng và không buộc duy trì Vue + React lâu dài.

Không migrate chỉ vì:

* Demo đẹp.
* Có nhiều animation.
* Giống sản phẩm khác.
* Codex dễ sinh code React hơn.

---

# 22. User testing

## 22.1. Tester

Tối thiểu:

* Hai developer từng dùng n8n.
* Một developer chưa dùng workflow tool.
* Một technical user.
* Một người mới dùng Goflow.

## 22.2. Test task

Tester thực hiện mà không được hướng dẫn trực tiếp:

### Task A

```text
Tạo Manual Trigger
→ HTTP Request
→ xem JSON output
```

### Task B

```text
Tạo Cron
→ HTTP Request
→ Telegram hoặc Discord
```

### Task C

```text
Tạo Manual Trigger
→ IF
→ hai nhánh
```

### Task D

Cố tình cấu hình sai credential và xác định nguyên nhân lỗi.

### Task E

Mở một execution cũ và tìm output của node thứ hai.

## 22.3. Thu thập

Ghi lại:

* Thời gian hoàn thành.
* Số lần bị kẹt.
* Thành phần không tìm thấy.
* Thuật ngữ không hiểu.
* Error message không hữu ích.
* Hành động bị chọn nhầm.
* Chỗ cần người hướng dẫn.

## 22.4. Điều kiện pass

Ít nhất 4/5 tester hoàn thành Task A và D mà không cần can thiệp trực tiếp.

Không coi UX hoàn thành chỉ vì người phát triển dự án sử dụng được.

---

# 23. Automated testing

## 23.1. Component tests

Cần kiểm tra:

* Node search.
* Node picker keyboard controls.
* Required field validation.
* Workflow dirty state.
* Save state.
* Execution selector.
* Status rendering.
* Input/output tabs.
* Error rendering.
* Credential missing state.

## 23.2. End-to-end tests

Ưu tiên Playwright hoặc công cụ tương đương.

Các flow bắt buộc:

1. Tạo blank workflow.
2. Add manual trigger.
3. Add HTTP node.
4. Connect nodes.
5. Cấu hình required fields.
6. Save.
7. Run.
8. Xem node output.
9. Mở execution history.
10. Replay hoặc retry.
11. Toggle active.
12. Tạo workflow từ template.

## 23.3. Visual regression

Thiết lập screenshot tests cho:

* App shell.
* Empty state.
* Node picker.
* Canvas.
* Selected node.
* Running node.
* Failed node.
* Properties inspector.
* Execution list.
* Credential state.

Không dùng visual regression để thay thế usability testing.

## 23.4. Backend compatibility

Frontend tests phải xác nhận:

* Không gọi engine trực tiếp.
* Không mở SQLite.
* Trigger UI có source `ui`.
* Execution status lấy từ backend.
* Error payload được xử lý đúng.
* Authentication vẫn hoạt động.
* Scoped permissions không bị bypass.

---

# 24. Milestone thực thi

## Milestone 1 — UX audit và foundation

Phải hoàn thành:

* Audit toàn bộ UI hiện tại.
* Vẽ user journey.
* Lập inventory component.
* Tạo design tokens.
* Tạo app shell mới.
* Bỏ navbar quá tải.
* Tạo workflow page.
* Tạo execution page.
* Tạo credential page.
* Thiết lập frontend test foundation.
* Thiết lập E2E smoke.

Definition of Done:

* Navigation rõ ràng.
* Không mất chức năng cũ.
* Frontend build pass.
* E2E mở app, chọn workflow và chạy workflow pass.

## Milestone 2 — Editor usability

Phải hoàn thành:

* Node picker có search.
* Quick-add.
* Node card mới.
* Edge cleanup.
* Dirty/saved state.
* Undo/redo.
* Auto-layout.
* Validation badges.
* Empty canvas onboarding.
* Template flow.

Definition of Done:

* Tester tìm và thêm node dưới 15 giây.
* Workflow ba node có thể tạo mà không dùng palette cố định.
* Không có edge animation khi idle.
* Invalid node được chỉ rõ trước khi chạy.

## Milestone 3 — Inspector và data mapping

Phải hoàn thành:

* Parameters/Input/Output/Logs.
* Required field validation.
* Credential-first configuration.
* JSON tree viewer.
* Copy path.
* Data picker.
* Expression mode.
* Resolved preview.
* Pinned data hoặc execution sample selection.

Definition of Done:

* Người dùng map output node A vào node B qua UI.
* Preview resolved value đúng.
* Không cần gõ expression path thủ công trong use case cơ bản.

## Milestone 4 — Execution debugger

Phải hoàn thành:

* Execution selector.
* Canvas execution overlay.
* Failed path highlight.
* Attempts/duration.
* Retry full workflow.
* Replay execution.
* Cancel execution.
* Debug bundle.
* Contextual error actions.

Definition of Done:

* Tester xác định node lỗi dưới 2 phút.
* Không phải mở raw database hoặc terminal để debug.
* Debug bundle không chứa secret.

## Milestone 5 — Node-level debugging

Phải đánh giá kỹ feasibility.

Mục tiêu:

* Run node.
* Run from here.
* Run until here.
* Retry failed node.
* Pin output.

Các tính năng chỉ được đánh dấu hoàn thành khi backend hỗ trợ behavior an toàn và có test.

Không giả lập bằng cách chạy toàn workflow nhưng hiển thị như chỉ chạy một node.

## Milestone 6 — Architecture decision

Sau user test:

* Đánh giá Vue Flow.
* Làm FlowGram prototype nếu cần.
* Chấm scorecard.
* Ghi quyết định trong ADR.
* Chọn giữ Vue Flow hoặc migrate.

Không migration trong âm thầm.

---

# 25. Non-goals

Không thực hiện trong UX GOAL này:

* Multi-tenant SaaS.
* Team RBAC.
* Billing.
* SSO.
* Distributed workers.
* Redis queue.
* PostgreSQL migration.
* Marketplace quy mô lớn.
* Mobile workflow editor đầy đủ.
* Clone nguyên giao diện n8n.
* Copy source code hoặc assets của sản phẩm khác.
* Rewrite backend engine.
* Thêm hàng trăm integration.
* Xây `goflow node init` và node SDK hoàn chỉnh.

Node development CLI và MCP sẽ nằm trong:

```text
DEVX_GOAL.md
```

sau khi UX editor và debugger đạt yêu cầu.

---

# 26. Release criteria

UX GOAL chưa hoàn thành nếu chỉ:

* Đổi màu.
* Đổi border radius.
* Đổi font.
* Thêm shadow.
* Thêm animation.
* Làm screenshot đẹp.
* Viết tài liệu.
* Thêm một vài template.
* Đổi Vue sang React.
* Chuyển sang FlowGram.
* Người phát triển tự thấy dễ dùng.

Trước khi release phải pass:

```bash
go test ./...
go vet ./...
go build ./...
```

Frontend:

```bash
cd ui
npm ci
npm run build
npm run test
```

E2E:

```bash
npm run test:e2e
```

Smoke:

* Windows binary.
* Linux binary.
* Embedded frontend.
* Workflow create/save/run.
* WebSocket execution state.
* Execution debugger.
* Credential-required node.
* Template workflow.
* Error case.
* Replay/retry.

---

# 27. Definition of Done cuối cùng

UX GOAL được xem là hoàn thành khi:

1. App shell có information architecture rõ ràng.
2. Workflows, Executions và Credentials không phụ thuộc vào modal làm navigation chính.
3. Node picker có search, category, keyboard và quick-add.
4. Node palette cố định không còn bắt buộc.
5. Canvas giảm visual noise.
6. Edge chỉ animate khi có execution liên quan.
7. Node card hiển thị configuration và execution state rõ.
8. Có workflow dirty/saved state.
9. Có undo/redo.
10. Có validation trước khi chạy.
11. Inspector có Parameters, Input, Output và Logs.
12. Required field và credential lỗi được hiển thị inline.
13. Người dùng map data giữa node qua UI.
14. Có expression preview.
15. Có execution selector.
16. Có failed path highlight.
17. Có retry và replay full workflow.
18. Có debug bundle đã redact.
19. Người dùng xác định lỗi node trong dưới 2 phút.
20. Người dùng mới tạo workflow đầu tiên trong dưới 10 phút.
21. Ít nhất 4/5 usability tester hoàn thành task chính.
22. Frontend component tests pass.
23. E2E tests pass.
24. Backend tests không regression.
25. Windows và Linux embedded binary smoke pass.
26. UI không bypass runtime security.
27. Có ADR quyết định giữ Vue Flow hoặc migrate.
28. Không có migration frontend lớn mà thiếu prototype và scorecard.
29. Tài liệu phản ánh đúng behavior đã test.
30. Không còn hạng mục code hoặc automated test khả thi bị bỏ qua với lý do “cần manual verification”.

---

# 28. Yêu cầu đối với Codex

Đọc toàn bộ:

```text
GOAL.md
UX_GOAL.md
GOAL_PROGRESS.md
README.md
```

Coi `GOAL.md` là runtime contract không được phá vỡ.

## 28.1. Bắt đầu bằng audit

Trước khi sửa code:

1. Audit toàn bộ frontend.
2. Liệt kê component hiện tại.
3. Vẽ user journey hiện tại.
4. Xác định điểm gây nhầm lẫn.
5. Kiểm tra các tính năng editor đã có.
6. Xác định phần có thể tái sử dụng.
7. Xác định phần cần refactor.
8. Không rewrite toàn bộ UI ngay.

Tạo file:

```text
UX_GOAL_PROGRESS.md
```

Mỗi mục gồm:

* Requirement.
* Status.
* Existing behavior.
* Implementation evidence.
* Automated test evidence.
* User-test evidence.
* Remaining work.
* Blocker.

Status:

```text
NOT_STARTED
IN_PROGRESS
BLOCKED
MANUAL_VERIFICATION_REQUIRED
DONE
```

## 28.2. Thực thi theo milestone

Thực hiện lần lượt:

```text
Milestone 1
→ Milestone 2
→ Milestone 3
→ Milestone 4
→ Milestone 5
→ Milestone 6
```

Không làm FlowGram migration trước Milestone 1–4.

## 28.3. Test-driven behavior

Với mỗi behavior:

1. Viết test hoặc E2E scenario.
2. Xác nhận test fail hoặc behavior chưa tồn tại.
3. Sửa code.
4. Chạy test liên quan.
5. Cập nhật `UX_GOAL_PROGRESS.md`.
6. Chỉ đánh dấu `DONE` khi behavior đã được kiểm chứng.

## 28.4. Không dừng ở kế hoạch

Không chỉ:

* Viết audit.
* Viết wireframe.
* Viết roadmap.
* Đổi CSS.
* Tạo mock component.
* Cập nhật documentation.

Phải tiếp tục thực thi code theo milestone.

## 28.5. Không phá runtime

Sau mỗi milestone chạy:

```bash
go test ./...
go vet ./...
go build ./...
```

Và:

```bash
cd ui
npm run build
npm run test
npm run test:e2e
```

## 28.6. Không tự tuyên bố hoàn thành

Không được đánh dấu UX GOAL hoàn thành nếu:

* Chưa có E2E.
* Chưa có usability test.
* Chưa test error flow.
* Chưa test workflow đầu tiên.
* Chưa test debugging.
* Chỉ có screenshot.
* Chỉ tester là chính người phát triển.
* Linux hoặc Windows embedded frontend chưa được kiểm tra.

## 28.7. Báo cáo sau mỗi milestone

Báo cáo:

* Files changed.
* Components created.
* Components removed.
* Behavior added.
* Tests added.
* Test commands and exact results.
* Screenshots hoặc recordings.
* User feedback.
* Known limitations.
* Next milestone.

## 28.8. Điều kiện được phép dừng

Chỉ dừng khi:

* Không còn công việc code hoặc automated test khả thi trong milestone hiện tại.
* Blocker phụ thuộc quyết định sản phẩm thực sự.
* Blocker được ghi rõ với lựa chọn và trade-off.
* Không dùng “manual verification pending” để bỏ qua code hoặc automated test vẫn làm được.

---

# 29. Báo cáo cuối cùng

Báo cáo cuối cùng phải gồm:

1. UX GOAL hoàn thành bao nhiêu phần trăm.
2. Definition of Done nào đã đạt.
3. Definition of Done nào chưa đạt.
4. Kết quả usability testing.
5. Time to first workflow.
6. Time to diagnose failure.
7. Files và component chính đã thay đổi.
8. Test mới.
9. Kết quả backend tests.
10. Kết quả frontend tests.
11. Kết quả E2E.
12. Kết quả Windows smoke.
13. Kết quả Linux smoke.
14. Quyết định Vue Flow hoặc FlowGram.
15. ADR liên quan.
16. Known limitations.
17. Rủi ro UX còn lại.
18. Công việc chuyển sang `DEVX_GOAL.md`.

---

# 30. Kết luận

Giai đoạn này không nhằm chứng minh rằng Goflow có nhiều tính năng hơn.

Mục tiêu là chứng minh rằng:

> Người dùng có thể hiểu, xây dựng, chạy và debug workflow mà không cần sự trợ giúp của người tạo ra Goflow.

Ưu tiên cuối cùng:

```text
Usability
→ Debuggability
→ Familiarity
→ Accessibility
→ Visual polish
```

Không đảo thứ tự thành:

```text
Visual polish
→ animation
→ framework rewrite
→ usability
```

Khi UX GOAL hoàn thành, Goflow phải đạt định vị:

> Một workflow automation editor quen thuộc, dễ debug, chạy bằng single Go binary và có nền tảng CLI/MCP mạnh cho developer.
