// https://github.com/jquery/jquery
// https://github.com/twbs/bootstrap
// https://github.com/aterrien/jQuery-Knob
// https://github.com/lokesh-coder/pretty-checkbox
// https://github.com/iconic/open-iconic

var commandProg;
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

$(document).ready(function() {
    $.ajaxSetup({
        cache : false
    });
    // nodes setup
    initNodes();
    // dials setup
    initDials('cs');
    initDials('sc');
    $('.dial').trigger('configure', dial_prop_all);

    setDefaults();
});

function command() {
    // suspend any pending commands
    $("#button_cmd").prop('disabled', true);
    $("#button_reset").prop('disabled', true);
    if (commandProg)
        clearInterval(commandProg);

    var i = 1;
    var activeApp = $('.nav-tabs .active > a').attr('name');
    $("#results").text("Executing ");
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
        $('#image_text').text('Execute camerapp to retrieve an image.');
    }

    console.info(JSON.stringify(form_data));
    $('#results').load('/command', form_data, function(resp, status, jqXHR) {
        console.info('resp:', resp);
        $(".stdout").scrollTop($(".stdout")[0].scrollHeight);
        $("#button_cmd").prop('disabled', false);
        $("#button_reset").prop('disabled', false);
        clearInterval(commandProg);
        // check for new images once, on command complete
        if (activeApp == "camerapp") {
            if (resp.includes('Done, exiting')) {
                setTimeout(function() {
                    $('#image_text').load('/txtlast', form_data);
                    $('#images').load('/imglast', form_data);
                }, 500);
            }
        }
    });
    // onsubmit should always return false to override native http call
    return false;
}

function initNodes() {
    loadNodes('cli');
    loadNodes('ser');
    $("a[data-toggle='tab']").on('shown.bs.tab', function(e) {
        updateNodeOptions('ser');
    });
    $('#sel_cli').change(function() {
        updateNode('cli');
    });
    $('#sel_ser').change(function() {
        updateNode('ser');
    });
}

function loadNodes(node) {
    var data = [ {
        name : "node_type",
        value : ((node == 'cli') ? "clients_default" : "servers_default")
    } ];
    console.info(JSON.stringify(data));
    $('#sel_' + node).load('/getnodes', data, function(resp, status, jqXHR) {
        console.info('resp:', resp);
        nodes[node] = JSON.parse(resp);
        updateNodeOptions(node);
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
        $('#ia_' + node).val(app_nodes[sel].isdas);
        $('#addr_' + node).val(app_nodes[sel].addr);
        $('#port_' + node).val(app_nodes[sel].port);
    }
}

function setDefaults() {
    if (commandProg) {
        clearInterval(commandProg);
    }
    $("#results").empty();
    $('#images').empty();
    $('#image_text').text('Execute camerapp to retrieve an image.');
    $('#stats_text').text('Execute sensorapp to retrieve sensor data.');
    $('#bwtest_text').text(
            'Dial values can be typed, edited, clicked, or scrolled to change.');

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
    $('#error_text').text(msg);
    $('#error_text').removeClass('enable');
    $('#error_text').addClass('enable');
    // remove animation once done
    $('#error_text').one('animationend', function(e) {
        $('#error_text').removeClass('enable');
        $('#error_text').text('');
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
