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
    RpcResponse            string
    IsSubmitSuccess        bool
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

    iscolumnexistsql := "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = '"+ blockinfo.AuxBlockTableName 
    iscolumnexistsql += "' and column_name = 'rpc_responce'"

    var count int
    result := handle.DbHandle.QueryRow(iscolumnexistsql).Scan(&count)
    if result != nil{
       glog.Info("Exec fail : ", iscolumnexistsql, result)
    }
    
    glog.Info("Number of rows are ", count)

    tx, err := handle.DbHandle.Begin()
    if err != nil{
        glog.Info("tx fail")
        return false
    }

    var sql string 
    if count != 0 {
        sql = "INSERT INTO " + blockinfo.AuxBlockTableName 
        sql += " (`bitcoin_block_hash`,`aux_block_hash`, `aux_pow`,`created_at`, `rpc_responce`) "
        sql += " values(?,?,?,?,?)"

        res, err := tx.Exec(sql,blockinfo.ParentChainBllockHash,blockinfo.AuxChainBlockHash,blockinfo.AuxPow, blockinfo.CurrentTime, blockinfo.RpcResponse)
        if err != nil{
            glog.Info("Exec fail : ", err)
            if blockinfo.IsSubmitSuccess {
                glog.Info("because auxblock submited successfully, we need update rpc_responce") 
                updatesql := "UPDATE "+ blockinfo.AuxBlockTableName + " SET rpc_responce=? WHERE aux_block_hash=?"
                _, fail := tx.Exec(updatesql, blockinfo.RpcResponse, blockinfo.AuxChainBlockHash)
                if fail != nil {
                   glog.Info( updatesql ,"Exec fail : ", fail)
                } else {
                    tx.Commit()
                    return true;
                }
            }
            tx.Commit()
            return false
        }

        glog.Info(res.LastInsertId())
    } else {
        sql = "INSERT INTO " + blockinfo.AuxBlockTableName 
        sql += " (`bitcoin_block_hash`,`aux_block_hash`, `aux_pow`,`created_at`) "
        sql += " values(?,?,?,?)"

        res, err := tx.Exec(sql,blockinfo.ParentChainBllockHash,blockinfo.AuxChainBlockHash,blockinfo.AuxPow, blockinfo.CurrentTime)
        if err != nil{
            glog.Info("Exec fail : ", err)
            tx.Commit()
            return false
        }

        glog.Info(res.LastInsertId())
    }

    tx.Commit()
    return true
}

// func main() {
// 	var config DBConnectionInfo
// 	config.Host = "127.0.0.1";
//     config.Port = "3306";
//     config.Username = "root";
//     config.Password = "";
//     config.Dbname = "bpool_local_db";

//     var dbhandle DBConnection
//     dbhandle.InitDB(config);

//     var info SubmitAuxBlockInfo
//     info.AuxBlockTableName = "found_doge_blocks"
//     info.ParentChainBllockHash = "a4ee7a37411ce2b50138148e70f7506d132556103e84e186c0da4a8e781812d6"
//     info.AuxChainBlockHash = "b4ee7a37411ce2b50138f48e70f7506d132556103e84e186c0da4a8e781812d6"
//     info.AuxPow = "a4ee7a37411ce2b50138f48e70f7506d132556103e84e186c0da4a8e781812d6"
//     info.CurrentTime = "2019-12-21 02:15:33"
//     info.RpcResponse = "{\"id\":0, \"result\": false}"
//     info.IsSubmitSuccess = true
//     // timeStr:=time.Now().Format("2006-01-02 15:04:05")  //当前时间的字符串，2006-01-02 15:04:05据说是golang的诞生时间，固定写法

//     // glog.Info(timeStr) 

//     dbhandle.InsertAuxBlock(info)
// }