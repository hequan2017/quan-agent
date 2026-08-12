const state = { messages: [], keys: [], machines: [], skills: [] };
const $ = (id) => document.getElementById(id);

async function request(url, options = {}) {
  const response = await fetch(url, { headers: { "Content-Type": "application/json" }, ...options });
  if (response.status === 204) return null;
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || `请求失败 (${response.status})`);
  return body;
}

function escapeHTML(value) { const node = document.createElement("span"); node.textContent = String(value || ""); return node.innerHTML; }
function card(html) { const node = document.createElement("div"); node.className = "item"; node.innerHTML = html; return node; }
function showError(error) { alert(error.message); }

async function loadSettings() {
  const settings = await request("/api/settings");
  $("baseUrl").value = settings.base_url; $("model").value = settings.model;
  $("keyDot").classList.toggle("ok", settings.key_configured);
  $("keyStatus").textContent = settings.key_configured ? "DeepSeek 已配置" : "未配置 DeepSeek";
  $("keyHint").textContent = settings.key_configured ? `${settings.masked_api_key} · ${settings.model}` : "配置 API Key 后即可对话";
}

async function loadKeys() {
  state.keys = await request("/api/keys");
  $("keys").replaceChildren(...state.keys.map((key) => card(`<h3>${escapeHTML(key.name)}</h3><p class="muted">创建于 ${escapeHTML(key.created_at)}</p><div class="actions"><button class="danger small" data-delete-key="${key.id}">删除</button></div>`)));
  $("machineKey").innerHTML = state.keys.map((key) => `<option value="${key.id}">${escapeHTML(key.name)}</option>`).join("");
  if (!state.keys.length) $("keys").append(card("<p class='muted'>还没有密钥。录入私钥后才能添加机器。</p>"));
}

async function loadMachines() {
  state.machines = await request("/api/machines"); const info = await request("/api/skills"); state.skills = info.skills;
  $("sshStatus").textContent = info.ssh_available ? "Windows OpenSSH 已就绪" : "未检测到 Windows OpenSSH Client";
  $("machines").replaceChildren(...state.machines.map((machine) => {
    const buttons = state.skills.map((skill) => `<button class="secondary small" data-run-skill="${skill.id}" data-machine="${machine.id}">${escapeHTML(skill.name)}</button>`).join("");
    const tags = (machine.tags || []).map((tag) => `<span class="badge">${escapeHTML(tag)}</span>`).join("");
    return card(`<h3>${escapeHTML(machine.name)}</h3><p class="muted">${escapeHTML(machine.user)}@${escapeHTML(machine.host)}:${machine.port}</p><div>${tags}</div><div class="actions">${buttons}<button class="danger small" data-delete-machine="${machine.id}">删除</button></div>`);
  }));
  if (!state.machines.length) $("machines").append(card("<p class='muted'>还没有机器。请先录入 SSH 密钥，再录入 Linux 机器。</p>"));
}

function addMessage(role, content) {
  state.messages.push({ role, content }); const node = document.createElement("div"); node.className = `message ${role}`; node.textContent = content; $("messages").appendChild(node); node.scrollIntoView({ behavior: "smooth", block: "end" });
}

document.querySelectorAll("[data-view]").forEach((button) => button.addEventListener("click", () => {
  document.querySelectorAll("[data-view]").forEach((item) => item.classList.toggle("active", item === button));
  document.querySelectorAll(".view").forEach((view) => view.classList.toggle("active", view.id === button.dataset.view));
  if (button.dataset.view === "fleetView") loadMachines().catch(showError);
  if (button.dataset.view === "keyView") loadKeys().catch(showError);
}));

$("openSettings").addEventListener("click", () => $("settingsDialog").showModal());
$("cancelSettings").addEventListener("click", () => $("settingsDialog").close());
$("openKey").addEventListener("click", () => $("keyDialog").showModal());
$("openMachine").addEventListener("click", async () => { await loadKeys(); if (!state.keys.length) return alert("请先录入一个 SSH 私钥"); $("machineDialog").showModal(); });
document.querySelectorAll(".close-dialog").forEach((button) => button.addEventListener("click", () => button.closest("dialog").close()));

$("settingsForm").addEventListener("submit", async (event) => { event.preventDefault(); $("settingsError").textContent = ""; try { await request("/api/settings", { method: "PUT", body: JSON.stringify({ api_key: $("apiKey").value, base_url: $("baseUrl").value, model: $("model").value }) }); $("apiKey").value = ""; await loadSettings(); $("settingsDialog").close(); } catch (error) { $("settingsError").textContent = error.message; } });
$("clearKey").addEventListener("click", async () => { if (!confirm("确认清除当前用户保存的 DeepSeek API Key？")) return; await request("/api/settings/key", { method: "DELETE" }); await loadSettings(); $("settingsDialog").close(); });
$("keyForm").addEventListener("submit", async (event) => { event.preventDefault(); $("keyError").textContent = ""; try { await request("/api/keys", { method: "POST", body: JSON.stringify({ name: $("keyName").value, private_key: $("privateKey").value }) }); $("keyForm").reset(); $("keyDialog").close(); await loadKeys(); } catch (error) { $("keyError").textContent = error.message; } });
$("machineForm").addEventListener("submit", async (event) => { event.preventDefault(); $("machineError").textContent = ""; const tags = $("machineTags").value.split(",").map((x) => x.trim()).filter(Boolean); try { await request("/api/machines", { method: "POST", body: JSON.stringify({ name: $("machineName").value, host: $("machineHost").value, port: Number($("machinePort").value), user: $("machineUser").value, key_id: $("machineKey").value, tags }) }); $("machineForm").reset(); $("machinePort").value = 22; $("machineUser").value = "root"; $("machineDialog").close(); await loadMachines(); } catch (error) { $("machineError").textContent = error.message; } });

document.addEventListener("click", async (event) => {
  try {
    if (event.target.dataset.deleteKey) { if (!confirm("确认删除此 SSH 密钥？")) return; await request(`/api/keys/${event.target.dataset.deleteKey}`, { method: "DELETE" }); await loadKeys(); }
    if (event.target.dataset.deleteMachine) { if (!confirm("确认删除此机器记录？不会操作远程机器。")) return; await request(`/api/machines/${event.target.dataset.deleteMachine}`, { method: "DELETE" }); await loadMachines(); }
    if (event.target.dataset.runSkill) { const button = event.target; button.disabled = true; $("skillOutput").textContent = "正在通过 SSH 执行只读 Skill..."; try { const result = await request(`/api/machines/${button.dataset.machine}/skills/${button.dataset.runSkill}`, { method: "POST", body: "{}" }); $("skillOutput").textContent = `[${result.skill_name}] ${result.duration_ms}ms\n\n${result.output}`; } finally { button.disabled = false; } }
  } catch (error) { if (event.target.dataset.runSkill) $("skillOutput").textContent = `执行失败：${error.message}`; else showError(error); }
});

$("chatForm").addEventListener("submit", async (event) => { event.preventDefault(); const content = $("prompt").value.trim(); if (!content) return; addMessage("user", content); $("prompt").value = ""; $("send").disabled = true; try { const result = await request("/api/chat", { method: "POST", body: JSON.stringify({ messages: state.messages }) }); addMessage("assistant", result.reply); } catch (error) { addMessage("assistant", `请求失败：${error.message}`); } finally { $("send").disabled = false; $("prompt").focus(); } });
$("prompt").addEventListener("keydown", (event) => { if (event.key === "Enter" && !event.shiftKey) { event.preventDefault(); $("chatForm").requestSubmit(); } });
Promise.all([loadSettings(), loadKeys(), loadMachines()]).catch((error) => addMessage("assistant", `初始化失败：${error.message}`));
