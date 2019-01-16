package main

import (
    "strings"
    "database/sql"

    _ "github.com/go-sql-driver/mysql"
    "github.com/golang/glog"
)
type DBConnection struct {
	DbHandle *sql.DB
}

type SubmitAuxBlockInfo struct {
	AuxBlockTableName      string
	ParentChainBllockHash  string
	AuxChainBlockHash      string
	AuxPow                 string
	CurrentTime            string
}


func (handle *DBConnection) InitDB(conf DBConnectionInfo) {

    
    path := strings.Join([]string{conf.Username, ":", conf.Password, "@tcp(",conf.Host, ":", conf.Port, ")/", conf.Dbname, "?charset=utf8"}, "")

    glog.Info("dbpath : " + path )

 
    handle.DbHandle, _ = sql.Open("mysql", path)
    
    handle.DbHandle.SetConnMaxLifetime(100)
    
    handle.DbHandle.SetMaxIdleConns(10)
    
    if err := handle.DbHandle.Ping(); err != nil{
        glog.Info("opon database fail : %s", err)
        return
    }

    glog.Info("connnect success")

}


func (handle *DBConnection) InsertAuxBlock(blockinfo SubmitAuxBlockInfo) (bool){

    tx, err := handle.DbHandle.Begin()
    if err != nil{
        glog.Info("tx fail")
        return false
    }

    sql := "INSERT INTO " + blockinfo.AuxBlockTableName 
    sql += " (`bitcoin_block_hash`,`aux_block_hash`, `aux_pow`,`created_at`) "
    sql += " values(?,?,?,?)"

    res, err := tx.Exec(sql,blockinfo.ParentChainBllockHash,blockinfo.AuxChainBlockHash,blockinfo.AuxPow, blockinfo.CurrentTime)
    if err != nil{
        glog.Info("Exec fail : %s", err)
        return false
    }

    tx.Commit()
    glog.Info(res.LastInsertId())
    return true
}

// func main() {
// 	var config DBConnectionInfo
// 	config.Host = "127.0.0.1";
//     config.Port = "3306";
//     config.Username = "root";
//     config.Password = "root";
//     config.Dbname = "bpool_local_db";

//     var dbhandle DBConnection
//     dbhandle.InitDB(config);

//     var info AuxBlockInfo
//     info.AuxBlockTableName = "found_nmc_blocks"
//     info.ParentChainBllockHash = "a4ee7a37411ce2b50138148e70f7506d132556103e84e186c0da4a8e781812d6"
//     info.AuxChainBlockHash = "a4ee7a37411ce2b50138f48e70f7506d132556103e84e186c0da4a8e781812d6"
//     info.AuxPow = "a4ee7a37411ce2b50138f48e70f7506d132556103e84e186c0da4a8e781812d6"
//     info.CurrentTime = "2019-12-21 02:15:33"

//     timeStr:=time.Now().Format("2006-01-02 15:04:05")  //当前时间的字符串，2006-01-02 15:04:05据说是golang的诞生时间，固定写法

//     glog.Info(timeStr) 

//     dbhandle.InsertAuxBlock(info)

// }