(function () {
  "use strict";

  var current = null;
  var statusOrder = [
    "evidence_collecting", "risk_assessed", "pilot_ready", "review_pending",
    "review_approved", "frozen", "credential_issued"
  ];
  var caseSelect = document.getElementById("caseSelect");
  var notice = document.getElementById("notice");

  function key(prefix) {
    return prefix + "-" + Date.now() + "-" + Math.random().toString(16).slice(2);
  }

  function split(value) {
    return String(value || "").split(",").map(function (item) { return item.trim(); }).filter(Boolean);
  }

  function localDate(input) {
    return new Date(input).toISOString();
  }

  function commandMeta(actor) {
    if (!current) { throw new Error("请先选择处置案"); }
    return { expectedVersion: current.version, idempotencyKey: key("web"), actor: actor };
  }

  async function api(path, options) {
    var config = options || {};
    config.headers = Object.assign({ "Accept": "application/json" }, config.headers || {});
    if (config.body) { config.headers["Content-Type"] = "application/json"; }
    var response = await fetch(path, config);
    var data = await response.json().catch(function () { return {}; });
    if (!response.ok) {
      var detail = data.error || {};
      throw new Error(detail.message || ("HTTP " + response.status));
    }
    return data;
  }

  function message(text, isError) {
    notice.textContent = text;
    notice.className = "notice show" + (isError ? " error" : "");
    window.clearTimeout(message.timer);
    message.timer = window.setTimeout(function () { notice.className = "notice"; }, 6500);
  }

  async function perform(label, action) {
    try {
      var result = await action();
      if (result && result.id) {
        current = result;
        caseSelect.value = result.id;
      }
      await refreshCurrent();
      message(label + "成功", false);
      return result;
    } catch (error) {
      message(error.message, true);
      throw error;
    }
  }

  function renderCurrent() {
    var meta = document.getElementById("caseMeta");
    if (!current) {
      meta.textContent = "请先建档或选择处置案";
      return;
    }
    meta.textContent = current.caveCode + " · " + current.muralZone + " · 状态 " + current.status + " · v" + current.version;
    var index = current.status === "remediation_required" ? 0 : statusOrder.indexOf(current.status);
    document.querySelectorAll("#progressSteps li").forEach(function (item, itemIndex) {
      item.classList.toggle("done", itemIndex < index);
      item.classList.toggle("active", itemIndex === index);
    });
    if (current.credential) {
      document.querySelector('#verifyForm [name="credentialNo"]').value = current.credential.credentialNo;
	  document.querySelector('#revokeForm [name="credentialNo"]').value = current.credential.credentialNo;
	  var active = current.credential.revocationStatus === "active";
      renderCredential({ valid: active, message: active ? "凭据已签发，可执行独立验真" : "凭据已撤销：" + current.credential.revocationReason, credential: current.credential, manifest: current.frozenManifest });
    }
	renderTrial();
    updateActionAvailability();
  }

  function updateActionAvailability() {
    var map = {
      evidenceForm: ["evidence_collecting", "remediation_required"],
      assess: ["evidence_collecting"],
      planForm: ["risk_assessed"],
	  trialStartForm: ["pilot_ready"],
	  trialObservationForm: ["pilot_ready"],
      reviewForm: ["review_pending"],
      freeze: ["review_approved"],
	  credentialForm: ["frozen"],
	  revokeForm: ["credential_issued"]
    };
    Object.keys(map).forEach(function (id) {
      var element = document.getElementById(id);
      var enabled = current && map[id].indexOf(current.status) >= 0;
      element.querySelectorAll ? element.querySelectorAll("button, input, select, textarea").forEach(function (control) { control.disabled = !enabled; }) : element.disabled = !enabled;
      if (element.tagName === "BUTTON") { element.disabled = !enabled; }
    });
	var latestTrial = current && current.trials && current.trials[current.trials.length - 1];
	if (latestTrial && current.status === "pilot_ready") {
	  document.querySelectorAll("#trialStartForm button, #trialStartForm input").forEach(function (control) { control.disabled = true; });
	} else if (!latestTrial) {
	  document.querySelectorAll("#trialObservationForm button, #trialObservationForm input, #trialObservationForm textarea").forEach(function (control) { control.disabled = true; });
	}
	if (current && current.credential && current.credential.revocationStatus !== "active") {
	  document.querySelectorAll("#revokeForm button, #revokeForm input, #revokeForm textarea").forEach(function (control) { control.disabled = true; });
	}
  }

  async function loadCases() {
    var response = await api("/api/v1/cases");
    var selected = current && current.id;
    caseSelect.innerHTML = '<option value="">选择处置案</option>';
    response.items.forEach(function (item) {
      var option = document.createElement("option");
      option.value = item.id;
      option.textContent = item.caveCode + " / " + item.muralZone + " · " + item.status;
      caseSelect.appendChild(option);
    });
    if (selected) { caseSelect.value = selected; }
  }

  async function refreshCurrent() {
    if (current && current.id) {
      current = await api("/api/v1/cases/" + encodeURIComponent(current.id));
      renderCurrent();
      await loadAudit();
	  await loadTrends();
    }
    await loadCases();
  }

  async function loadTrends() {
	var view = document.getElementById("trendView");
	if (!current) { return; }
	var response = await api("/api/v1/cases/" + encodeURIComponent(current.id) + "/evidence/trends");
	view.innerHTML = "";
	if (!response.items.length) {
	  view.className = "trend-view empty";
	  view.textContent = "当前案卷尚无证据修订";
	  return;
	}
	view.className = "trend-view";
	response.items.forEach(function (trend) {
	  var zone = document.createElement("div");
	  zone.className = "trend-zone";
	  var title = document.createElement("strong");
	  title.textContent = trend.zoneCode + " · 最新风险 " + trend.latestRiskLevel + " · 总体 " + trend.overallDirection;
	  zone.appendChild(title);
	  trend.revisions.forEach(function (point) {
		var line = document.createElement("small");
		var delta = point.coverageDelta === null ? "基线" : "覆盖 " + signed(point.coverageDelta) + "% / 活性 " + signed(point.activityDelta);
		line.textContent = "r" + point.revision + " · " + new Date(point.recordedAt).toLocaleString("zh-CN") + " · 覆盖 " + point.coveragePercent + "% · 活性 " + point.activityScore + " · 风险 " + point.riskLevel + " · " + delta + " · " + point.conclusion + " · 原因 " + ((point.riskReasonCodes || []).join("、") || "无");
		zone.appendChild(line);
	  });
	  view.appendChild(zone);
	});
  }

  function signed(value) { return (value > 0 ? "+" : "") + Number(value).toFixed(1); }

  function renderTrial() {
	var view = document.getElementById("trialView");
	var latest = current && current.trials && current.trials[current.trials.length - 1];
	if (!latest) {
	  view.className = "trial-view empty";
	  view.textContent = "尚未建立试验";
	  return;
	}
	view.className = "trial-view";
	view.innerHTML = "";
	var title = document.createElement("strong");
	title.textContent = latest.plotCode + " · 修订 r" + latest.revision + " · " + latest.windowStatus;
	view.appendChild(title);
	var summary = document.createElement("small");
	summary.textContent = "末次色差 " + latest.colorDelta + " · 活性降低 " + latest.activityReduction.toFixed(1) + "% · 阻断原因 " + ((latest.windowReasonCodes || []).join("、") || "无");
	view.appendChild(summary);
	(latest.observations || []).forEach(function (observation) {
	  var line = document.createElement("small");
	  line.textContent = observation.hoursSinceStart + "h · " + new Date(observation.observedAt).toLocaleString("zh-CN") + " · 色差 " + observation.colorDelta + " · 活性 " + observation.activityScore;
	  view.appendChild(line);
	});
  }

  async function loadAudit() {
    if (!current) { return; }
    var response = await api("/api/v1/cases/" + encodeURIComponent(current.id) + "/audit");
    var timeline = document.getElementById("timeline");
    timeline.innerHTML = "";
    response.items.forEach(function (event) {
      var li = document.createElement("li");
      var strong = document.createElement("strong");
      strong.textContent = event.sequence + " · " + event.summary;
      var small = document.createElement("small");
      small.textContent = new Date(event.occurredAt).toLocaleString("zh-CN") + " · " + event.actor + " · 案件 v" + event.caseVersion + " · " + event.eventHash.slice(0, 12);
      li.appendChild(strong);
      li.appendChild(small);
      timeline.appendChild(li);
    });
  }

  function renderCredential(result) {
    var view = document.getElementById("credentialView");
    var credential = result.credential;
    view.className = "credential";
    view.innerHTML = "";
    [
      result.valid ? "✓ " + result.message : "× " + result.message,
      "凭据编号：" + credential.credentialNo,
      "开放范围：" + credential.allowedZones.join("、"),
      "有效期至：" + new Date(credential.validUntil).toLocaleDateString("zh-CN"),
      "冻结摘要：" + credential.frozenManifestDigest,
      "签名摘要：" + credential.signatureDigest
    ].forEach(function (text, index) {
      var line = document.createElement(index === 0 ? "strong" : "div");
      line.textContent = text;
      view.appendChild(line);
    });
	if (credential.revocationStatus !== "active") {
	  var revoked = document.createElement("div");
	  revoked.textContent = "撤销：" + credential.revocationReason + " · " + credential.revokedBy + " · " + new Date(credential.revokedAt).toLocaleString("zh-CN");
	  view.appendChild(revoked);
	}
  }

  document.getElementById("createForm").addEventListener("submit", function (event) {
    event.preventDefault();
    var data = new FormData(event.currentTarget);
    perform("处置案建档", async function () {
      current = await api("/api/v1/cases", {
        method: "POST",
        body: JSON.stringify({
          idempotencyKey: key("create"),
          caveCode: data.get("caveCode"),
          muralZone: data.get("muralZone"),
          materialSensitivity: data.get("materialSensitivity"),
          discoveredAt: localDate(data.get("discoveredAt")),
          owner: data.get("owner")
        })
      });
      return current;
    }).catch(function () {});
  });

  document.getElementById("evidenceForm").addEventListener("submit", function (event) {
    event.preventDefault();
    var data = new FormData(event.currentTarget);
    perform("证据修订提交", function () {
      var payload = Object.assign(commandMeta("保护技术员"), {
        zoneCode: data.get("zoneCode"),
        samplePoints: split(data.get("samplePoints")),
        microscopyFinding: data.get("microscopyFinding"),
        cultureFinding: data.get("cultureFinding"),
        imageDigest: data.get("imageDigest"),
        coveragePercent: Number(data.get("coveragePercent")),
        activityScore: Number(data.get("activityScore"))
      });
      return api("/api/v1/cases/" + current.id + "/evidence", { method: "POST", body: JSON.stringify(payload) });
    }).catch(function () {});
  });

  document.getElementById("assess").addEventListener("click", function () {
    perform("风险评估", function () {
      var payload = Object.assign(commandMeta("微生物评估人员"), { assessor: "微生物评估人员" });
      return api("/api/v1/cases/" + current.id + "/assessment", { method: "POST", body: JSON.stringify(payload) });
    }).catch(function () {});
  });

  document.getElementById("planForm").addEventListener("submit", function (event) {
    event.preventDefault();
    var data = new FormData(event.currentTarget);
    perform("处置方案提交", function () {
      var zones = {};
      (current.evidence || []).forEach(function (item) { zones[item.zoneCode] = true; });
      if (!Object.keys(zones).length) { zones[data.get("zoneCode")] = true; }
      var payload = Object.assign(commandMeta("微生物评估人员"), {
        zoneInstructions: Object.keys(zones).map(function (zoneCode) {
          return {
            zoneCode: zoneCode,
            cleaningMedium: data.get("cleaningMedium"),
            concentration: Number(data.get("concentration")),
            contactMinutes: Number(data.get("contactMinutes"))
          };
        }),
        isolationMeasures: split(data.get("isolationMeasures")),
        stopConditions: split(data.get("stopConditions")),
        rationale: data.get("rationale")
      });
      return api("/api/v1/cases/" + current.id + "/plans", { method: "POST", body: JSON.stringify(payload) });
    }).catch(function () {});
  });

  document.getElementById("trialStartForm").addEventListener("submit", function (event) {
    event.preventDefault();
    var data = new FormData(event.currentTarget);
	perform("小区试验基线建立", function () {
	  var plan = current.plans[current.plans.length - 1];
	  var payload = Object.assign(commandMeta("保护技术员"), {
		planVersion: plan.version,
		plotCode: data.get("plotCode"),
		startedAt: localDate(data.get("startedAt")),
		baseline: data.get("baseline"),
		baselineActivity: Number(data.get("baselineActivity"))
	  });
	  return api("/api/v1/cases/" + current.id + "/trials/start", { method: "POST", body: JSON.stringify(payload) });
	}).catch(function () {});
  });

  document.getElementById("trialObservationForm").addEventListener("submit", function (event) {
	event.preventDefault();
	var data = new FormData(event.currentTarget);
	perform("分期观察追加", function () {
	  var trial = current.trials[current.trials.length - 1];
	  var plan = current.plans[current.plans.length - 1];
      var deviations = [];
      if (data.get("deviationCode")) {
        deviations.push({ code: data.get("deviationCode"), description: data.get("deviationDescription") });
      }
      var payload = Object.assign(commandMeta("保护技术员"), {
		trialId: trial.id,
		planVersion: plan.version,
		observation: {
		  observedAt: localDate(data.get("observedAt")),
		  hoursSinceStart: Number(data.get("hoursSinceStart")),
		  colorDelta: Number(data.get("colorDelta")),
		  activityScore: Number(data.get("activityScore")),
		  note: data.get("note")
		},
        deviations: deviations
      });
	  return api("/api/v1/cases/" + current.id + "/trials/observations", { method: "POST", body: JSON.stringify(payload) });
    }).catch(function () {});
  });

  document.getElementById("reviewForm").addEventListener("submit", function (event) {
    event.preventDefault();
    var data = new FormData(event.currentTarget);
    perform("复核裁决", function () {
      var trial = current.trials[current.trials.length - 1];
      var decision = data.get("decision");
      var deviations = (trial.deviations || []).map(function (item) {
        return {
          code: item.code,
          description: item.description,
          decision: decision === "approve" ? "accept" : "reject",
          resolution: decision === "reject" ? "补充证据、修订方案并重新试验" : "风险可接受"
        };
      });
      var payload = Object.assign(commandMeta("保护责任复核员"), {
        reviewer: data.get("reviewer"),
        decision: decision,
        notes: data.get("notes"),
        deviations: deviations
      });
      return api("/api/v1/cases/" + current.id + "/review", { method: "POST", body: JSON.stringify(payload) });
    }).catch(function () {});
  });

  document.getElementById("freeze").addEventListener("click", function () {
    perform("证据清单冻结", function () {
      var payload = Object.assign(commandMeta("保护责任复核员"), { frozenBy: "保护责任复核员" });
      return api("/api/v1/cases/" + current.id + "/freeze", { method: "POST", body: JSON.stringify(payload) });
    }).catch(function () {});
  });

  document.getElementById("credentialForm").addEventListener("submit", function (event) {
    event.preventDefault();
    var data = new FormData(event.currentTarget);
    perform("开放安全凭据签发", async function () {
      var payload = Object.assign(commandMeta("保护责任复核员"), {
        allowedZones: split(data.get("allowedZones")),
        conditions: split(data.get("conditions")),
        issuedBy: data.get("issuedBy"),
        validUntil: new Date(data.get("validUntil") + "T23:59:59").toISOString()
      });
      var result = await api("/api/v1/cases/" + current.id + "/credentials", { method: "POST", body: JSON.stringify(payload) });
      document.querySelector('#verifyForm [name="credentialNo"]').value = result.credential.credentialNo;
      return result;
    }).catch(function () {});
  });

  document.getElementById("revokeForm").addEventListener("submit", function (event) {
	event.preventDefault();
	var data = new FormData(event.currentTarget);
	perform("开放安全凭据撤销", function () {
	  var payload = Object.assign(commandMeta(data.get("actor")), {
		credentialNo: data.get("credentialNo"),
		reason: data.get("reason")
	  });
	  return api("/api/v1/cases/" + current.id + "/credentials/revoke", { method: "POST", body: JSON.stringify(payload) });
	}).catch(function () {});
  });

  document.getElementById("verifyForm").addEventListener("submit", function (event) {
    event.preventDefault();
    var number = new FormData(event.currentTarget).get("credentialNo");
	fetch("/api/v1/credentials/" + encodeURIComponent(number) + "/verify", { headers: { "Accept": "application/json" } })
	  .then(function (response) { return response.json(); })
	  .then(function (result) {
		if (!result.credential) { throw new Error((result.error && result.error.message) || "验真失败"); }
		renderCredential(result);
		message(result.message, !result.valid);
	  })
	  .catch(function (error) { message(error.message, true); });
  });

  caseSelect.addEventListener("change", function () {
    if (!caseSelect.value) { current = null; renderCurrent(); return; }
    api("/api/v1/cases/" + encodeURIComponent(caseSelect.value))
	  .then(function (result) { current = result; renderCurrent(); return Promise.all([loadAudit(), loadTrends()]); })
      .catch(function (error) { message(error.message, true); });
  });
  document.getElementById("reload").addEventListener("click", function () { refreshCurrent().catch(function (error) { message(error.message, true); }); });

  var now = new Date();
  now.setMinutes(now.getMinutes() - now.getTimezoneOffset());
  document.querySelector('#createForm [name="discoveredAt"]').value = now.toISOString().slice(0, 16);
	document.querySelector('#trialStartForm [name="startedAt"]').value = now.toISOString().slice(0, 16);
	document.querySelector('#trialObservationForm [name="observedAt"]').value = now.toISOString().slice(0, 16);
  var valid = new Date();
  valid.setFullYear(valid.getFullYear() + 1);
  document.querySelector('#credentialForm [name="validUntil"]').value = valid.toISOString().slice(0, 10);
  loadCases().catch(function (error) { message(error.message, true); });
  updateActionAvailability();
}());
