package topic

const (
	KeyFileScanFile         = "topic::file::scan::file"
	KeyFileClearFile        = "topic::file::clear::file"
	KeyFileBatchDelete      = "topic::file::batch_delete::file"
	KeyFileDelete           = "topic::file::delete::file"
	KeyFileBatchModifyToken = "topic::file::batch_modify_token"

	KeyAutoIngestRefreshSubscribe = "topic::autoingest::refresh::subscribe"

	// KeyMediaClear 清除媒体文件
	KeyMediaClear = "topic::media::clear"
	// KeyMediaRebuildStrmFile 重建媒体文件 strm 文件
	KeyMediaRebuildStrmFile = "topic::media::rebuild::strm::file"
	// KeyMediaRebuildStrmFileByMountPoint 单个挂载点STRM重建
	KeyMediaRebuildStrmFileByMountPoint = "topic::media::rebuild::strm::file::by::mountpoint"
)
