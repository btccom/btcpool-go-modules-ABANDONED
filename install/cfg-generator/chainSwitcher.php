#!/usr/bin/env php
<?php
require_once __DIR__.'/lib/init.php';
// PHP syntax for templates
// https://www.php.net/manual/control-structures.alternative-syntax.php
// https://www.php.net/manual/language.basic-syntax.phpmode.php

$c = [
    "Kafka" => [],
    "MySQL" => [],
];

$c['Kafka']['Brokers'] = commaSplitTrim('KafkaBrokers');
if (empty($c['Kafka']['Brokers']) || in_array('', $c['Kafka']['Brokers'])) {
    fatal('KafkaBrokers cannot be empty');
}
$c['Kafka']['ControllerTopic'] = notNullTrim("KafkaControllerTopic");
$c['Kafka']['ProcessorTopic'] = notNullTrim("KafkaProcessorTopic");


$c['Algorithm'] = notNullTrim("Algorithm");
$c['ChainDispatchAPI'] = notNullTrim("ChainDispatchAPI");
$c['SwitchIntervalSeconds'] = (int)optionalTrim('SwitchIntervalSeconds', 60);

$c['FailSafeChain'] = notNullTrim("FailSafeChain");
$c['FailSafeSeconds'] = (int)optionalTrim('FailSafeSeconds', $c['SwitchIntervalSeconds'] * 10);

$c['ChainNameMap'] = json_decode(notNullTrim('ChainNameMap'), true);
if ($c['ChainNameMap'] === null) {
    fatal("wrong JSON in ChainNameMap: $error\n$json");
}
if (empty($c['ChainNameMap']) || in_array('', $c['ChainNameMap'])) {
    fatal('ChainNameMap cannot be empty');
}

$c['MySQL']['ConnStr'] = notNullTrim("MySQLConnStr");
$c['MySQL']['Table'] = optionalTrim('MySQLTable', 'chain_switcher_record');

echo toJSON($c);
