(() => {
  const $ = id => document.getElementById(id);
  const orderSelect = $("order"), data = $("order-data"), message = $("message"), form = $("receiver");
  let selected;

  const api = async (path, init = {}) => {
    const response = await fetch(path, init);
    if (response.ok) return response;
    let problem = {title:"Не удалось выполнить запрос"};
    try { problem = await response.json(); } catch (_) {}
    const error = new Error(problem.title || "Ошибка запроса"); error.problem = problem; throw error;
  };
  const tell = (text, kind = "") => { message.hidden = !text; message.textContent = text; message.className = `panel ${kind}`; };
  const money = value => {
    const [whole, fraction = ""] = String(value).split(".");
    return `${whole},${(fraction + "00").slice(0, 2)} ₽`;
  };
  const receiver = () => Object.fromEntries(new FormData(form).entries());
  const problemText = e => {
    if (e.problem?.errors) return Object.entries(e.problem.errors).map(([k,v]) => `${fieldName(k)}: ${v}`).join("\n");
    return e.problem?.detail || e.problem?.title || e.message;
  };
  const fieldName = key => ({lastName:"Фамилия",firstName:"Имя",middleName:"Отчество",phone:"Телефон",email:"Почта",gender:"Пол"}[key] || key);
  const clearFieldErrors = () => {
    form.querySelectorAll("input").forEach(input => {
      input.classList.remove("invalid");
      input.removeAttribute("aria-invalid");
    });
    form.querySelectorAll("[data-error-for]").forEach(el => el.textContent = "");
  };
  const showFieldErrors = errors => Object.entries(errors || {}).forEach(([field, text]) => {
    const input = form.elements[field], error = form.querySelector(`[data-error-for="${field}"]`);
    if (input) { input.classList.add("invalid"); input.setAttribute("aria-invalid", "true"); }
    if (error) error.textContent = text;
  });
  function formatPhone(value) {
    let digits = value.replace(/\D/g, "");
    // Оставляем префикс на экране уже после первой цифры, но не съедаем
    // введённую «8»/«7»: иначе при ручном наборе поле очищалось на первом
    // символе и до полного номера было невозможно добраться.
    if (digits === "8" || digits === "7") return "+7 (";
    if (digits.startsWith("8") || digits.startsWith("7")) digits = digits.slice(1);
    digits = digits.slice(0, 10);
    if (!digits) return "";
    let out = "+7";
    if (digits.length) out += ` (${digits.slice(0, 3)}`;
    if (digits.length >= 3) out += ")";
    if (digits.length > 3) out += ` ${digits.slice(3, 6)}`;
    if (digits.length > 6) out += `-${digits.slice(6, 8)}`;
    if (digits.length > 8) out += `-${digits.slice(8, 10)}`;
    return out;
  }
  form.elements.phone.addEventListener("input", event => { event.target.value = formatPhone(event.target.value); });
  form.querySelectorAll("input").forEach(input => input.addEventListener("input", () => {
    input.classList.remove("invalid"); input.removeAttribute("aria-invalid");
    const error = form.querySelector(`[data-error-for="${input.name}"]`);
    if (error) error.textContent = "";
  }));

  const stepper = $("stepper");
  const stepperItems = [...stepper.querySelectorAll(".stepper-item")];
  const stepperConnectors = [...stepper.querySelectorAll(".stepper-connector")];
  const STEP_STATES = {
    select: ["active", "upcoming", "upcoming"],
    review: ["done", "active", "upcoming"],
    done: ["done", "done", "done"],
  };
  function updateStepper(stage) {
    const states = STEP_STATES[stage];
    stepperItems.forEach((item, i) => {
      item.classList.remove("is-active", "is-done", "is-upcoming");
      item.classList.add(`is-${states[i]}`);
    });
    stepperConnectors.forEach((line, i) => line.classList.toggle("is-filled", states[i] === "done"));
  }

  function render(sheet) {
    updateStepper("review");
    $("title").textContent = `Заказ ${sheet.order.number} от ${new Date(sheet.order.date).toLocaleDateString("ru-RU")}`;
    $("customer").textContent = `${sheet.customer || "Клиент не найден"} · ${sheet.order.warehouse}`;
    $("lines").innerHTML = sheet.lines.map(l => `<tr><td>${escape(l.article)}</td><td>${escape(l.name)}</td><td>${l.qty}</td><td>${money(l.price)}</td><td>${escape(l.vatName)}</td><td>${money(l.amount)}</td><td>${money(l.vatAmount)}</td><td>${money(l.amountWithVat)}</td><td>${stockCell(l.stock)}</td></tr>`).join("");
    $("totals").innerHTML = `<tr><td colspan="5">Итого</td><td>${money(sheet.totals.amount)}</td><td>${money(sheet.totals.vatAmount)}</td><td>${money(sheet.totals.amountWithVat)}</td><td></td></tr>`;
    const suggestion = sheet.suggestedReceiver;
    if (suggestion) ["lastName","firstName","middleName"].forEach(k => form.elements[k].value = suggestion[k] || "");
    data.hidden = false; $("print").disabled = !sheet.issuable;
    $("order-note").textContent = sheet.issuable ? "" : `Выдача недоступна: ${sheet.blockReasons.join(", ")}`;
    $("order-note").className = sheet.issuable ? "hint" : "hint blocked";
  }
  function escape(value) { const el = document.createElement("span"); el.textContent = value; return el.innerHTML; }
  // stockCell — «сколько на складе», плюс нехватка и чужие резервы отдельно:
  // остаток — это не «доступно к выдаче», два понятия смешивать нельзя (ADR 0003).
  function stockCell(stock) {
    const parts = [stock.onHand];
    if (stock.shortage !== "0") parts.push(`не хватает ${stock.shortage}`);
    if (stock.reservedByOthers !== "0") parts.push(`резерв: ${stock.reservedByOthers}`);
    return parts.length > 1 ? `${parts[0]} (${parts.slice(1).join(", ")})` : parts[0];
  }
  async function loadOrder() {
    const ref = orderSelect.value; if (!ref) return; tell("");
    try { selected = ref; render(await (await api(`/api/v1/orders/${encodeURIComponent(ref)}`)).json()); }
    catch (e) { data.hidden = true; tell(problemText(e), "error"); }
  }
  form.addEventListener("submit", async event => {
    event.preventDefault(); if (!selected) return;
    clearFieldErrors();
    $("print").disabled = true; $("print").classList.add("loading"); $("print-label").textContent = "Формирую документ…";
    const body = JSON.stringify({orderRef:selected, customer:receiver()});
    try {
      const response = await api("/api/v1/documents/pdf", {method:"POST",headers:{"Content-Type":"application/json"},body});
      const blob = await response.blob(), url = URL.createObjectURL(blob), link = document.createElement("a");
      const contentDisposition = response.headers.get("Content-Disposition") || "";
      const match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i);
      link.href=url; link.download=match ? decodeURIComponent(match[1]) : "pickup.pdf"; link.click(); URL.revokeObjectURL(url);
      const draft = await (await api(`/api/v1/orders/${encodeURIComponent(selected)}/shipment-draft`, {method:"PUT"})).json();
      tell(`PDF сформирован. Черновик реализации ${draft.number} записан и проверен в 1С.`, "success");
      updateStepper("done");
    } catch (e) {
      if (e.problem?.errors) showFieldErrors(e.problem.errors);
      else tell(problemText(e), "error");
    }
    finally { $("print").disabled = false; $("print").classList.remove("loading"); $("print-label").textContent = "Сформировать документ"; }
  });
  (async () => {
    try {
      const orders = await (await api("/api/v1/orders")).json();
      orderSelect.innerHTML = `<option value="">Выберите заказ</option>` + orders.map(o => `<option value="${o.ref}" ${o.issuable ? "" : "disabled"}>${escape(o.number)} · ${escape(o.customer || "клиент не найден")}${o.issuable ? "" : " — недоступен"}</option>`).join("");
      orderSelect.disabled = false;
    } catch (e) { tell(problemText(e), "error"); }
  })();
  orderSelect.addEventListener("change", loadOrder);
})();
