// https://github.com/jquery/jquery
// https://github.com/d3/d3
// https://github.com/twbs/bootstrap
// https://github.com/aterrien/jquery-knob
// https://github.com/lokesh-coder/pretty-checkbox
// https://github.com/iconic/open-iconic
// https://github.com/turuslan/hacktimer
// https://code.highcharts.com

var commandProg;
var intervalGraphTick;
var secMax = 10;
var secMin = 1;
var sizeMax = 1400; // TODO: pull from MTU
var sizeMin = 64;
var pktMax = 187500000;
var pktMin = 1;
var bwMax = 150.00;
var bwMin = 0.0000001;
var feedback = {};
var nodes = {};

var granularity = 7;
var ticks = 20 * granularity;
var tickMs = 1000 / granularity;
var xLeftTrimMs = 1000 / granularity;
var bwIntervalBufMs = 1000;
var chartCS;
var chartSC;
var hasFocus = true;
var lastTime;

var dial_prop_all = {
    // dial constants
    'width' : '100',
    'height' : '100',
    "angleOffset" : "-125",
    "angleArc" : "250",
    "inputColor" : "#000",
    "lineCap" : "default",
};
var dial_prop_arc = {
    'cursor' : true,
    'fgColor' : '#f00',
    'bgColor' : '#fff',
};
var dial_prop_text = {
    'fgColor' : '#0000', // opaque
    'bgColor' : '#0000', // opaque
};

// instruction information
var bwText = 'Dial values can be typed, edited, clicked, or scrolled to change.';
var imageText = 'Execute camerapp to retrieve an image.';
var sensorText = 'Execute sensorapp to retrieve sensor data.';
var bwgraphsText = 'Click legend to hide/show data when continuous test is on.';
var cont_disable_msg = 'Continuous testing disabled.'

// results data extraction regex
var reSCHdr = /(s->c results)/i;
var reCSHdr = /(c->s results)/i;
var reBwAtt = /(?:attempted bandwidth:\s*)([0-9.-]*)(?:\s*bps)/i;
var reBwAch = /(?:achieved bandwidth:\s*)([0-9.-]*)(?:\s*bps)/i;
var reItVar = /(?:interarrival time variance:\s*)([0-9.-]*)(?:\s*ms)/i;
var reItMin = /(?:interarrival time min:\s*)([0-9.-]*)(?:\s*ms)/i;
var reItAvg = /(?:average interarrival time:\s*)([0-9.-]*)(?:\s*ms)/i;
var reItMax = /(?:interarrival time max:\s*)([0-9.-]*)(?:\s*ms)/i;
var reErr1 = /(?:err=*)(["'])(?:(?=(\\?))\2.)*?\1/i;
var reErr2 = /(?:crit msg=*)(["'])(?:(?=(\\?))\2.)*?\1/i;
var reErr3 = /(?:error:\s*)([\s\S]*)/i;

$(document).ready(function() {
    $.ajaxSetup({
        cache : false,
        timeout : 30000,
    });
    // nodes setup
    initNodes();
    // dials setup
    initDials('cs');
    initDials('sc');
    $('.dial').trigger('configure', dial_prop_all);
    initBwGraphs();

    setDefaults();
});

$(window).blur(function() {
    // on lost focus, throttle down graph drawing
    hasFocus = false;
    console.debug('window focus:', hasFocus);
    clearInterval(intervalGraphTick);
    granularity = 1;
    manageTickData();
});

$(window).focus(function() {
    // on lost focus, throttle up graph drawing
    hasFocus = true;
    console.debug('window focus:', hasFocus);
    clearInterval(intervalGraphTick);
    granularity = 7;
    manageTickData();
});

function initBwGraphs() {
    // continuous test default: off
    $('#switch_cont').prop("checked", false);
    $('#bwtest-graphs').css("display", "block");
    // continuous test switch
    $('#switch_cont').change(function() {
        var checked = $(this).prop('checked');
        if (checked) {
            enableTestControls(false);
            lockTab("bwtester");
            // starts continuous tests
            command(true);
        } else if (!commandProg) {
            enableTestControls(true);
            releaseTabs();
        }
    });

    var checked = $('#switch_utc').prop('checked');
    setChartUtc(checked);
    $('#switch_utc').change(function() {
        var checked = $(this).prop('checked');
        setChartUtc(checked);
    });

    updateBwInterval();

    // charts update on tab switch
    $('a[data-toggle="tab"]').on('shown.bs.tab', function(e) {
        var activeApp = $('.nav-tabs .active > a').attr('name');
        var isBwtest = (activeApp == "bwtester");
        // show/hide graphs for bwtester
        $('#bwtest-graphs').css("display", isBwtest ? "block" : "none");
        var checked = $('#switch_cont').prop('checked');
        if (checked && !isBwtest) {
            $("#switch_cont").prop('checked', false);
            enableTestControls(true);
            releaseTabs();
            show_temp_err(cont_disable_msg);
        }
    });
    // setup charts
    var csColAch = $('#svg-client circle').css("fill");
    var scColAch = $('#svg-server circle').css("fill");
    var csColReq = $('#svg-cs line').css("stroke");
    var scColReq = $('#svg-sc line').css("stroke");
    chartCS = drawBwtestSingleDir('cs', 'upload (mbps)', true, csColReq,
            csColAch);
    chartSC = drawBwtestSingleDir('sc', 'download (mbps)', true, scColReq,
            scColAch);

    lastTime = (new Date()).getTime() - (ticks * tickMs) + xLeftTrimMs;
    manageTickData();
}

function setChartUtc(useUTC) {
    Highcharts.setOptions({
        global : {
            useUTC : useUTC
        }
    });
}

function getBwParamDisplay() {
    return getBwParamLine('cs') + ' / ' + getBwParamLine('sc');
}

function getBwParamLine(dir) {
    return dir + ': ' + $('#dial-' + dir + '-sec').val() + 's, '
            + $('#dial-' + dir + '-size').val() + 'b x '
            + $('#dial-' + dir + '-pkt').val() + ' pkts, '
            + $('#dial-' + dir + '-bw').val() + ' Mbps';
}

function drawBwtestSingleDir(dir, yAxisLabel, legend, reqCol, achCol) {
    var div_id = dir + "-bwtest-graph";
    var chart = Highcharts.chart(div_id, {
        chart : {
            type : 'scatter',
            animation : Highcharts.svg,
            marginRight : 10,
        },
        title : {
            text : null
        },
        xAxis : {
            type : 'datetime',
            tickPixelInterval : 150,
            crosshair : true,
        },
        yAxis : [ {
            title : {
                text : yAxisLabel
            },
            gridLineWidth : 1,
            min : 0,
        } ],
        tooltip : {
            enabled : true,
            formatter : formatTooltip,
        },
        legend : {
            y : -15,
            layout : 'horizontal',
            align : 'right',
            verticalAlign : 'top',
            floating : true,
            enabled : legend,
        },
        credits : {
            enabled : false,
        },
        exporting : {
            enabled : false
        },
        plotOptions : {},
        series : [ {
            name : 'attempted',
            data : loadSetupData(),
            color : reqCol,
            marker : {
                symbol : 'triangle-down'
            },
        }, {
            name : 'achieved',
            data : loadSetupData(),
            color : achCol,
            marker : {
                symbol : 'triangle'
            },
            dataLabels : {
                enabled : true,
                formatter : function() {
                    return Highcharts.numberFormat(this.y, 2)
                },
            },
        } ]
    });
    return chart;
}

function formatTooltip() {
    var tooltip = '<b>' + this.series.name + '</b><br/>'
            + Highcharts.dateFormat('%Y-%m-%d %H:%M:%S', this.x) + '<br/>'
            + Highcharts.numberFormat(this.y, 2) + ' mbps';
    if (this.point.error != null) {
        tooltip += '<br/><b>' + this.point.error + '</b>';
    }
    return tooltip;
}

function loadSetupData() {
    // points are a function of timeline speed (width & seconds)
    // no data points on setup
    var data = [], time = (new Date()).getTime(), i;
    for (i = -ticks; i <= 0; i += 1) {
        data.push({
            x : time + i * tickMs,
            y : null
        });
    }
    return data;
}

function manageTickData() {
    // add placeholders for time ticks
    ticks = 20 * granularity;
    tickMs = 1000 / granularity;
    xLeftTrimMs = 1000 / granularity;
    intervalGraphTick = setInterval(function() {
        var newTime = (new Date()).getTime();
        refreshTickData(chartCS, newTime);
        refreshTickData(chartSC, newTime);
    }, tickMs);
}

function refreshTickData(chart, newTime) {
    var x = newTime, y = null;
    var series0 = chart.series[0];
    var series1 = chart.series[1];
    var shift = false;

    lastTime = x - (ticks * tickMs) + xLeftTrimMs;
    // manually remove all left side ticks < left side time
    // wait for adding hidden ticks to draw
    var draw = false;
    removeOldPoints(series0, lastTime, draw);
    removeOldPoints(series1, lastTime, draw);
    // manually add hidden right side ticks, time = now
    // do all drawing here to avoid accordioning redraws
    // do not shift points since we manually remove before this
    draw = true;
    series0.addPoint([ x, y ], draw, shift);
    series1.addPoint([ x, y ], draw, shift);
}

function removeOldPoints(series, lastTime, draw) {
    for (var i = 0; i < series.data.length; i++) {
        if (series.data[i].x < lastTime) {
            series.removePoint(i, draw);
        } else {
            break; // only need to query left-most data
        }
    }
}

function updateBwGraph(data, time) {
    updateBwChart(chartCS, data.cs, time);
    updateBwChart(chartSC, data.sc, time);
}

function updateBwChart(chart, dataDir, time) {
    var bw = dataDir.bandwidth / 1000000;
    var tp = dataDir.throughput / 1000000;
    var loss = dataDir.throughput / dataDir.bandwidth;
    // manually add visible right side ticks, time = now
    // wait for adding hidden ticks to draw, for consistancy
    // do not shift points since we manually remove before this
    var draw = false;
    var shift = false;
    if (dataDir.error) {
        chart.series[0].addPoint({
            x : time,
            y : bw,
            error : dataDir.error,
            color : '#f00',
            marker : {
                symbol : 'diamond',
            }
        }, draw, shift);
    } else {
        chart.series[0].addPoint([ time, bw ], draw, shift);
    }
    chart.series[1].addPoint([ time, tp ], draw, shift);
}

function endProgress() {
    clearInterval(commandProg);
    commandProg = false;
}

function command(continuous) {
    var startTime = (new Date()).getTime();

    // suspend any pending commands
    if (commandProg) {
        endProgress();
    }
    var i = 1;
    var activeApp = $('.nav-tabs .active > a').attr('name');
    enableTestControls(false);
    lockTab(activeApp);
    if (!continuous) {
        $("#results").empty();
    }
    $("#results").append("Executing ");
    $('#results').append(activeApp);
    $('#results').append(" client");
    commandProg = setInterval(function() {
        $('#results').append('.');
        i += 1;
    }, 500);

    // add required client/server address options
    var form_data = $('#command-form').serializeArray();
    form_data.push({
        name : "apps",
        value : activeApp
    });
    if (activeApp == "bwtester") {
        // add extra bwtester options required
        form_data.push({
            name : "bw_cs",
            value : formatBwtestCmd('-cs', 'cs')
        }, {
            name : "bw_sc",
            value : formatBwtestCmd('-sc', 'sc')
        });
    }
    if (activeApp == "camerapp") {
        // clear for new image request
        $('#images').empty();
        $('#image_text').text(imageText);
    }

    console.info(JSON.stringify(form_data));
    $('#results').load('/command', form_data, function(resp, status, jqXHR) {
        console.info('resp:', resp);
        $(".stdout").scrollTop($(".stdout")[0].scrollHeight);
        endProgress();
        // continuous flag should force switch

        if (activeApp == "camerapp") {
            // check for new images once, on command complete
            handleImageResponse(resp);
        } else if (activeApp == "bwtester") {
            // check for usable data for graphing
            handleBwResponse(resp, continuous, startTime);
        } else {
            handleGeneralResponse();
        }
    });
    // onsubmit should always return false to override native http call
    return false;
}

function enableTestControls(enable) {
    $("#button_cmd").prop('disabled', !enable);
    $("#button_reset").prop('disabled', !enable);
    $("#addl_opt").prop('disabled', !enable);
}

function lockTab(href) {
    enableTab("bwtester", "bwtester" == href);
    enableTab("camerapp", "camerapp" == href);
    enableTab("sensorapp", "sensorapp" == href);
}

function releaseTabs() {
    enableTab("bwtester", true);
    enableTab("camerapp", true);
    enableTab("sensorapp", true);
}

function enableTab(href, enable) {
    if (enable) {
        $('.nav-tabs a[href="#' + href + '"]').attr("data-toggle", "tab");
        $('.nav-tabs a[href="#' + href + '"]').parent('li').removeClass(
                'disabled');
    } else {
        $('.nav-tabs a[href="#' + href + '"]').removeAttr('data-toggle');
        $('.nav-tabs a[href="#' + href + '"]').parent('li')
                .addClass('disabled');
    }
}

function handleGeneralResponse() {
    enableTestControls(true);
    releaseTabs();
}

function handleImageResponse(resp) {
    if (resp.includes('Done, exiting')) {
        setTimeout(function() {
            $('#image_text').load('/txtlast');
            $('#images').load('/imglast');
        }, 500);
    }
    enableTestControls(true);
    releaseTabs();
}

function handleBwResponse(resp, continuous, startTime) {
    var endTime = (new Date()).getTime();
    var data = extractBwtestRespData(resp);
    console.log(JSON.stringify(data));

    // TODO: log parsed data to metrics for papers

    // provide parsed data to graph
    updateBwGraph(data, endTime);

    // check for continuous testing
    var checked = $('#switch_cont').prop('checked');
    if (checked) {
        var cs = $('#dial-cs-sec').val() * 1000;
        var sc = $('#dial-sc-sec').val() * 1000;
        var cont = $('#bwtest_sec').val() * 1000;
        var diff = endTime - startTime;
        var max = Math.max(cs, sc, cont);
        var interval = max > diff ? max - diff : 0;
        console.log('Test took ' + diff + 'ms, max: ' + max
                + 'ms, waiting another ' + interval + 'ms.');
        setTimeout(function() {
            var checked = $('#switch_cont').prop('checked');
            if (checked) {
                command(continuous);
            }
        }, interval);
    } else {
        enableTestControls(true);
        releaseTabs();
    }
}

function updateBwInterval() {
    var cs = $('#dial-cs-sec').val() * 1000;
    var sc = $('#dial-sc-sec').val() * 1000;
    var cont = $('#bwtest_sec').val() * 1000;
    var max = Math.max(cs, sc);
    if (cont != (max + bwIntervalBufMs)) {
        $('#bwtest_sec').val((max + bwIntervalBufMs) / 1000);
    }
    // update interval minimum
    var min = Math.min(cs, sc);
    $('#bwtest_sec').prop('min', min / 1000);
}

function extractBwtestRespData(resp) {
    var dir = null;
    var err = null;
    var data = {
        'cs' : {},
        'sc' : {},
    };
    r = resp.split("\n");
    for (var i = 0; i < r.length; i++) {
        if (r[i].match(reSCHdr)) {
            dir = 'sc';
        }
        if (r[i].match(reCSHdr)) {
            dir = 'cs';
        }
        if (r[i].match(reBwAtt)) {
            data[dir]['bandwidth'] = Number(r[i].match(reBwAtt)[1]);
        }
        if (r[i].match(reBwAch)) {
            data[dir]['throughput'] = Number(r[i].match(reBwAch)[1]);
        }
        if (r[i].match(reItVar)) {
            data[dir]['arrival_var'] = Number(r[i].match(reItVar)[1]);
        }
        if (r[i].match(reItMin)) {
            data[dir]['arrival_min'] = Number(r[i].match(reItMin)[1]);
        }
        if (r[i].match(reItAvg)) {
            data[dir]['arrival_avg'] = Number(r[i].match(reItAvg)[1]);
        }
        if (r[i].match(reItMax)) {
            data[dir]['arrival_max'] = Number(r[i].match(reItMax)[1]);
        }
        // evaluate error message potential
        if (r[i].match(reErr1)) {
            err = r[i].match(reErr1)[0];
        } else if (r[i].match(reErr2)) {
            err = r[i].match(reErr2)[0];
        } else if (r[i].match(reErr3)) {
            err = r[i].match(reErr3)[1];
        } else if (!err && r[i].trim().length != 0) {
            // fallback to first line if err msg needed
            err = r[i].trim();
        }
    }
    // update with errors, if any
    updateBwErrors(data.cs, 'cs', err);
    updateBwErrors(data.sc, 'sc', err);
    return data;
}

function updateBwErrors(dataDir, dir, err) {
    if (!dataDir.throughput || !dataDir.bandwidth) {
        dataDir.error = err;
        dataDir.bandwidth = parseFloat(get_bw(dir));
    }
}

function initNodes() {
    loadNodes('cli', 'clients_default');
    $("a[data-toggle='tab']").on('shown.bs.tab', function(e) {
        updateNodeOptions('ser');
    });
    $('#sel_cli').change(function() {
        updateNode('cli');
        // after client selection, update server options
        loadServerNodes();
    });
    $('#sel_ser').change(function() {
        updateNode('ser');
    });
}

function loadServerNodes() {
    // client 'lo' localhost interface selected, use localhost servers
    var name = $('#sel_cli').find("option:selected").html();
    if (name == "lo") {
        loadNodes('ser', "servers_user");
    } else {
        loadNodes('ser', "servers_default");
    }
}

function loadNodes(node, list) {
    var data = [ {
        name : "node_type",
        value : list
    } ];
    console.info(JSON.stringify(data));
    $('#sel_' + node).load('/getnodes', data, function(resp, status, jqXHR) {
        console.info('resp:', resp);
        nodes[node] = JSON.parse(resp);
        updateNodeOptions(node);
        if (node == 'cli') {
            // after client selection, update server options
            loadServerNodes();
        }
    });
}

function updateNodeOptions(node) {
    var activeApp = (node == 'cli') ? 'all' : $('.nav-tabs .active > a').attr(
            'name');
    console.debug(activeApp);
    var app_nodes = nodes[node][activeApp];
    $('#sel_' + node).empty();
    for (var i = 0; i < app_nodes.length; i++) {
        $('#sel_' + node).append($('<option>', {
            value : i,
            text : app_nodes[i].name
        }));
    }
    updateNode(node);
}

function updateNode(node) {
    // populate fields
    if (nodes[node]) {
        var activeApp = (node == 'cli') ? 'all' : $('.nav-tabs .active > a')
                .attr('name');
        var app_nodes = nodes[node][activeApp];
        var sel = $('#sel_' + node).find("option:selected").attr('value');
        $('#ia_' + node).val(app_nodes[sel].isdas.replace(/_/g, ":"));
        $('#addr_' + node).val(app_nodes[sel].addr);
        $('#port_' + node).val(app_nodes[sel].port);
    }
}

function setDefaults() {
    if (commandProg) {
        endProgress();
    }
    $("#results").empty();
    $('#images').empty();
    $('#image_text').text(imageText);
    $('#stats_text').text(sensorText);
    $('#bwtest_text').text(bwText);
    $('#bwgraphs_text').text(bwgraphsText);

    onchange_radio('cs', 'size');
    onchange_radio('sc', 'size');

    updateNode('cli');
    updateNode('ser');
}

function formatBwtestCmd(arg, dir) {
    return arg + '=' + $('#dial-' + dir + '-sec').val() + ','
            + $('#dial-' + dir + '-size').val() + ','
            + $('#dial-' + dir + '-pkt').val() + ',' + parseInt(get_bw(dir))
            + 'bps';
}

function extend(obj, src) {
    for ( var key in src) {
        if (src.hasOwnProperty(key))
            obj[key] = src[key];
    }
    return obj;
}

function initDials(dir) {
    $('input[type=radio][name=' + dir + '-dial]').on('change', function() {
        onchange_radio(dir, $(this).val());
    });

    var prop_sec = {
        'min' : secMin,
        'max' : secMax,
        'release' : function(v) {
            return onchange(dir, 'sec', v);
        },
    };
    var prop_size = {
        'min' : 1, // 1 allows < 64 to be typed
        'max' : sizeMax,
        'release' : function(v) {
            return onchange(dir, 'size', v);
        },
    };
    var prop_pkt = {
        'min' : pktMin,
        'max' : pktMax,
        'release' : function(v) {
            return onchange(dir, 'pkt', v);
        },
        'draw' : function() {
            // allow large font when possible
            var pkt = $('#dial-' + dir + '-pkt').val();
            if (pkt < 999999) {
                $(this.i).css("font-size", "16px");
            } else if (pkt < 9999999) {
                $(this.i).css("font-size", "13px");
            } else {
                $(this.i).css("font-size", "11px");
            }
        },
    };
    var prop_bw = {
        'min' : 0.01, // 0.01 works around library typing issues
        'max' : bwMax,
        'step' : 0.01,
        'release' : function(v) {
            return onchange(dir, 'bw', v);
        },
        'format' : function(v) {
            // native formatting occasionally uses full precision
            // so we format it manually ourselves
            return Number(Math.round(v + 'e' + 2) + 'e-' + 2);
        },
    };
    $('#dial-' + dir + '-sec').knob(extend(prop_sec, dial_prop_arc));
    $('#dial-' + dir + '-size').knob(extend(prop_size, dial_prop_arc));
    $('#dial-' + dir + '-pkt').knob(extend(prop_pkt, dial_prop_text));
    $('#dial-' + dir + '-bw').knob(extend(prop_bw, dial_prop_arc));
}

function onchange_radio(dir, value) {
    console.debug('radio change ' + dir + '-' + value);
    // change read-only status, based on radio change
    switch (value) {
    case 'size':
        setDialLock(dir, 'size', true);
        setDialLock(dir, 'pkt', false);
        setDialLock(dir, 'bw', false);
        break;
    case 'pkt':
        setDialLock(dir, 'size', false);
        setDialLock(dir, 'pkt', true);
        setDialLock(dir, 'bw', false);
        break;
    case 'bw':
        setDialLock(dir, 'size', false);
        setDialLock(dir, 'pkt', false);
        setDialLock(dir, 'bw', true);
        break;
    }
}

function onchange(dir, name, v) {
    // change other dials, when dial values change
    console.debug('? ' + dir + '-' + name + ' ' + 'change' + ':', v);
    if (!feedback[dir + '-' + name]) {
        var lock = $('input[name=' + dir + '-dial]:checked').val();
        switch (name) {
        case 'sec':
            onchange_sec(dir, v, secMin, secMax, lock);
            break;
        case 'size':
            onchange_size(dir, v, sizeMin, sizeMax, lock);
            break;
        case 'pkt':
            onchange_pkt(dir, v, pktMin, pktMax, lock);
            break;
        case 'bw':
            onchange_bw(dir, v, bwMin, bwMax, lock);
            break;
        }
    } else {
        feedback[dir + '-' + name] = false;
    }
}

function update(dir, name, val, min, max) {
    var valid = (val <= max && val >= min);
    if (valid) {
        setTimeout(function() {
            console.debug('> ' + dir + '-' + name + ' trigger blur:', val);
            feedback[dir + '-' + name] = true;
            $('#dial-' + dir + '-' + name).val(val);
            $('#dial-' + dir + '-' + name).trigger('blur')
        }, 1);
    } else {
        console.warn('!> invalid ' + dir + '-' + name + ':', val);
        show_range_err(dir, name, val, min, max);
    }
    return valid;
}

function src_in_range(dir, name, v, min, max) {
    var value = v;
    if (v < min) {
        value = min;
    } else if (v > max) {
        value = max;
    }
    if (value != v) {
        console.warn('!~ invalid ' + dir + '-' + name + ':', v);
        show_range_err(dir, name, v, min, max);
        setTimeout(function() {
            console.debug('~ ' + dir + '-' + name + ' trigger blur:', value);
            $('#dial-' + dir + '-' + name).val(value);
            $('#dial-' + dir + '-' + name).trigger('blur')
        }, 1);
        return false;
    } else {
        return true;
    }
}

function show_range_err(dir, name, v, min, max) {
    var n = '';
    switch (name) {
    case 'sec':
        n = 'seconds';
        break;
    case 'size':
        n = 'packet size';
        break;
    case 'pkt':
        n = 'packets';
        break;
    case 'bw':
        n = 'bandwidth';
        break;
    }
    show_temp_err('Dial reset. It would cause the ' + n
            + ' dial to exceed its limit of ' + parseInt(max) + '.');
}

function show_temp_err(msg) {
    $('#error_text').removeClass('enable')
    $('#error_text').addClass('enable').text(msg);
    // remove animation once done
    $('#error_text').one('animationend', function(e) {
        $('#error_text').removeClass('enable').text('');
    });
}

function onchange_sec(dir, v, min, max, lock) {
    // changed sec, so update bw, else pkt
    if (src_in_range(dir, 'sec', v, min, max)) {
        var valid = true;
        switch (lock) {
        case 'size':
            valid = update_bw(dir);
            break;
        case 'pkt':
            valid = update_bw(dir);
            break;
        case 'bw':
            valid = update_pkt(dir);
            break;
        }
        if (!valid) {
            update_sec(dir);
        }
    }
    // special case: update continuous interval
    updateBwInterval();
}

function onchange_size(dir, v, min, max, lock) {
    // changed size, so update non-locked pkt/bw, feedback on max
    if (src_in_range(dir, 'size', v, min, max)) {
        var valid = true;
        switch (lock) {
        case 'size':
            valid = update_size(dir);
            break;
        case 'pkt':
            valid = update_bw(dir);
            break;
        case 'bw':
            valid = update_pkt(dir);
            break;
        }
        if (!valid) {
            update_size(dir);
        }
    }
}

function onchange_pkt(dir, v, min, max, lock) {
    // changed pkt, so update non-locked size/bw, feedback on max
    if (src_in_range(dir, 'pkt', v, min, max)) {
        var valid = true;
        switch (lock) {
        case 'size':
            valid = update_bw(dir);
            break;
        case 'pkt':
            valid = update_pkt(dir);
            break;
        case 'bw':
            valid = update_size(dir);
            break;
        }
        if (!valid) {
            update_pkt(dir);
        }
    }
}

function onchange_bw(dir, v, min, max, lock) {
    // changed bw, so update non-locked size/pkt, feedback on max
    if (src_in_range(dir, 'bw', v, min, max)) {
        var valid = true;
        switch (lock) {
        case 'size':
            valid = update_pkt(dir);
            break;
        case 'pkt':
            valid = update_size(dir);
            break;
        case 'bw':
            valid = update_bw(dir);
            break;
        }
        if (!valid) {
            update_bw(dir);
        }
    }
}

function update_sec(dir) {
    var val = parseInt(get_sec(dir) / 1000000);
    return update(dir, 'sec', val, secMin, secMax);
}

function update_size(dir) {
    var val = parseInt(get_size(dir) * 1000000);
    return update(dir, 'size', val, sizeMin, sizeMax);
}

function update_pkt(dir) {
    var val = parseInt(get_pkt(dir) * 1000000);
    return update(dir, 'pkt', val, pktMin, pktMax);
}

function update_bw(dir) {
    var val = parseFloat(get_bw(dir) / 1000000);
    return update(dir, 'bw', val, bwMin, bwMax);
}

function get_sec(dir) {
    return $('#dial-' + dir + '-pkt').val() * $('#dial-' + dir + '-size').val()
            * 8 / $('#dial-' + dir + '-bw').val();
}

function get_size(dir) {
    return $('#dial-' + dir + '-bw').val() / $('#dial-' + dir + '-pkt').val()
            * $('#dial-' + dir + '-sec').val() / 8;
}

function get_pkt(dir) {
    return $('#dial-' + dir + '-bw').val() / $('#dial-' + dir + '-size').val()
            * $('#dial-' + dir + '-sec').val() / 8;
}

function get_bw(dir) {
    return $('#dial-' + dir + '-pkt').val() * $('#dial-' + dir + '-size').val()
            / $('#dial-' + dir + '-sec').val() * 8;
}

function setDialLock(dir, value, readOnly) {
    var radioId = dir + '-radio-' + value;
    var dialId = 'dial-' + dir + '-' + value;
    $("#" + radioId).prop("checked", readOnly);
    $("#" + dialId).prop("readonly", readOnly);
    $("#" + dialId).prop("disabled", readOnly);
    $("#" + dialId).trigger('configure', {
        "readOnly" : readOnly ? "true" : "false",
        "inputColor" : readOnly ? "#999" : "#000",
    });
    if (readOnly) {
        console.debug(dialId + ' locked');
    }
}
