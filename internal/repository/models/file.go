package models

import (
	"time"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/datatypes"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"gorm.io/gorm"
)

type OsType = string

const (
	// OsTypeFolder 虚拟目录 用于目录显示和计算长度
	OsTypeFolder OsType = "folder"

	OsTypeFile OsType = "file"

	OsTypeSubscribe OsType = "subscribe"

	OsTypeSubscribeShareFolder OsType = "subscribe_share_folder"
	OsTypeSubscribeShareFile   OsType = "subscribe_share_file"

	OsTypeShareFolder OsType = "share_folder"
	OsTypeShareFile   OsType = "share_file"

	OsTypePersonFolder OsType = "person_folder"
	OsTypePersonFile   OsType = "person_file"

	OsTypeFamilyFolder OsType = "family_folder"
	OsTypeFamilyFile   OsType = "family_file"
)

type VirtualFile struct {
	ID         int64             `gorm:"primaryKey" json:"id"`
	CloudId    string            `gorm:"column:cloud_id;type:varchar(64);not null;default:'0'" json:"cloudId"`                                                   // 云端的文件ID
	ParentId   int64             `gorm:"column:parent_id;type:bigint;not null;default:0;uniqueIndex:parent_name_unique" json:"parentId"`                         // 上级文件ID
	TopId      int64             `gorm:"column:top_id;type:bigint;not null;default:0;index:idx_top_id" json:"topId"`                                             // 隶属于的挂载点ID（便于快速删除和查询） 如果本身是挂载点，那么 top_id = id
	IsTop      bool              `gorm:"column:is_top;type:boolean;default:false" json:"isTop"`                                                                  // 是否最顶层文件夹
	IsDir      bool              `gorm:"column:is_dir;type:boolean;default:false" json:"isDir"`                                                                  // 是否为目录
	Name       string            `gorm:"column:name;type:varchar(1024);not null;uniqueIndex:parent_name_unique" json:"name"`                                     // 文件名
	Size       int64             `gorm:"column:size;type:bigint;default:0" json:"size"`                                                                          // 文件大小
	Hash       string            `gorm:"column:hash;type:varchar(64);default:''" json:"hash"`                                                                    // 文件的hash值 这个没有啥用 考虑是否删除
	OsType     OsType            `gorm:"column:os_type;type:varchar(20);default:'folder'" json:"osType"`                                                         // 读取文件的方式
	Addition   datatypes.JSONMap `gorm:"column:addition;type:json" json:"addition"`                                                                              // 额外信息 例如分享id 文件夹id 等
	Rev        string            `gorm:"column:rev;type:varchar(64);default:''" json:"rev"`                                                                      // 版本 用于下次扫描时知道当前文件是删除还是修改还是新增
	IsDelete   int8              `gorm:"column:is_delete;type:smallint;default:0" json:"-"`                                                                      // 删除标记
	CreateDate time.Time         `gorm:"column:create_date;type:timestamp;default:CURRENT_TIMESTAMP" json:"createDate"`                                          // 云盘记录的创建时间
	ModifyDate time.Time         `gorm:"column:modify_date;type:timestamp;default:CURRENT_TIMESTAMP" json:"modifyDate"`                                          // 云盘记录的修改时间
	CreatedAt  time.Time         `gorm:"column:created_at;autoCreateTime;type:timestamp;default:CURRENT_TIMESTAMP" json:"createdAt"`                             // 数据库记录的创建时间
	UpdatedAt  time.Time         `gorm:"column:updated_at;autoUpdateTime;type:timestamp;default:CURRENT_TIMESTAMP;on update:CURRENT_TIMESTAMP" json:"updatedAt"` // 数据库记录的修改时间
}

func (f *VirtualFile) BeforeCreate(_ *gorm.DB) (err error) {
	f.Name = utils.SanitizeFileName(f.Name)

	if f.Addition == nil {
		f.Addition = datatypes.JSONMap{}
	}

	return
}

func (f *VirtualFile) BeforeUpdate(_ *gorm.DB) (err error) {
	f.Name = utils.SanitizeFileName(f.Name)

	return
}

func (f *VirtualFile) GetAddition(key string) any {
	return f.Addition[key]
}

const (
	rootName = "根目录"
)

func RootFile() *VirtualFile {
	return &VirtualFile{
		Name:       rootName,
		IsDir:      true,
		IsTop:      true,
		OsType:     OsTypeFolder,
		ParentId:   0,
		ModifyDate: time.Now(),
		CreateDate: time.Now(),
	}
}

func (f *VirtualFile) TableName() string {
	return "virtual_files"
}
