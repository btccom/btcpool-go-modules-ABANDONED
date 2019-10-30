#!/usr/bin/env php
<?php
require_once __DIR__.'/lib/init.php';
// PHP syntax for templates
// https://www.php.net/manual/control-structures.alternative-syntax.php
// https://www.php.net/manual/language.basic-syntax.phpmode.php

$c = [];

$c['AvailableCoins'] = commaSplitTrim('AvailableCoins');
if (empty($c['AvailableCoins']) || in_array('', $c['AvailableCoins'])) {
    fatal('AvailableCoins cannot be empty');
}

$c['UserListAPI'] = [];
foreach ($c['AvailableCoins'] as $coin) {
    $c['UserListAPI'][$coin] = notNullTrim("UserListAPI_$coin");
}

$c['IntervalSeconds'] = (int)optionalTrim('IntervalSeconds', 10);

$c['ZKBroker'] = commaSplitTrim('ZKBroker');
if (empty($c['ZKBroker']) || in_array('', $c['ZKBroker'])) {
    fatal('ZKBroker cannot be empty');
}

$c['ZKSwitcherWatchDir'] = notNullTrim("ZKSwitcherWatchDir");
$c['EnableUserAutoReg'] = isTrue('EnableUserAutoReg');

if ($c['EnableUserAutoReg']) {
    $c['ZKAutoRegWatchDir'] = notNullTrim("ZKAutoRegWatchDir");
    $c['UserAutoRegAPI'] = [
        'IntervalSeconds' => (int)optionalTrim('UserAutoRegAPI_IntervalSeconds', 10),
        'URL' => notNullTrim('UserAutoRegAPI_URL'),
        'User' => notNullTrim('UserAutoRegAPI_User'),
        'Password' => notNullTrim('UserAutoRegAPI_Password'),
        'DefaultCoin' => notNullTrim('UserAutoRegAPI_DefaultCoin'),
        'PostData' => json_decode(notNullTrim('UserAutoRegAPI_PostData')),
    ];

    if (!in_array($c['UserAutoRegAPI']['DefaultCoin'], $c['AvailableCoins'])) {
        fatal('cannot find UserAutoRegAPI_DefaultCoin in AvailableCoins');
    }
}

$c['StratumServerCaseInsensitive'] = isTrue('StratumServerCaseInsensitive');
$c['ZKUserCaseInsensitiveIndex'] = optionalTrim('ZKUserCaseInsensitiveIndex');

$c['EnableAPIServer'] = isTrue('EnableAPIServer');
if ($c['EnableAPIServer']) {
    $c['ListenAddr'] = notNullTrim("ListenAddr");
    $c['APIUser'] = optionalTrim('APIUser');
    $c['APIPassword'] = optionalTrim('APIPassword');
}

$c['EnableCronJob'] = isTrue('EnableCronJob');
if ($c['EnableCronJob']) {
    $c['CronIntervalSeconds'] = (int)optionalTrim('CronIntervalSeconds', 60);
    $c['UserCoinMapURL'] = notNullTrim("UserCoinMapURL");
}

echo toJSON($c);
