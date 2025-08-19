/**
 * Copyright 2017 Thibault Chataigner <thibault.chataigner@gmail.com>
 * Copyright 2024-2025 NetCracker Technology Corporation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

const regexpMetric = /([a-zA-Z_:][a-zA-Z0-9_:]*)(?:{(.*)})?\s+((?:\d*\.)?\d+(?:e\d+)?)(?:\s+(\d+))?/;
const regexpLabels = /([a-zA-Z_][a-zA-Z0-9_]*)\s?=\s?"([^"\\\\]*(?:\\.[^"\\\\]*)*)"/gm;

function parseLabels(metricName, rawLabelsStr) {
    let labels = {"__name__": metricName};
    if (rawLabelsStr !== undefined) {
        /*eslint no-cond-assign: "warn"*/
        while (match = regexpLabels.exec(rawLabelsStr)) {
            labels[match[1]] = match[2];
        }
    }
    return labels;
}

function parseSample(txtLine, defaultTimestampS) {
    let match = regexpMetric.exec(txtLine);
    if (match != null) {
        let labels = parseLabels(match[1], match[2]);
        let ts = parseInt(match[4]) || defaultTimestampS;
        return {"metric": labels, "value": [ts, match[3]]};
    }
    return null;
}

function handleSimulationResult(result) {
    let jsonres = JSON.parse(result);

    let html = "<dl>";
    $.each(jsonres, function (writerName, writerMsg) {
        html += '<dt>' + writerName + '</dt>';
        html += '<dd><pre class="alert alert-light">' + writerMsg + '</pre></dd>';
    });
    html += "</dl>";
    $("#outputs").html(html);
}

/*eslint no-unused-vars: "warn"*/
function simulWrite() {
    let txt = $("#input").val();
    let lines = txt.split(/\n/);
    let nowS = $.now() / 1000;

    let samples = [];
    $.each(lines, function (i, line) {
        let sample = parseSample($.trim(line), nowS);
        if (sample != null) {
            samples.push(sample);
        }
    });
    $.ajax({
        url: 'write',
        type: 'post',
        data: JSON.stringify(samples),
        headers: {"Content-Type": 'application/json'},
        success: handleSimulationResult
    });
}

