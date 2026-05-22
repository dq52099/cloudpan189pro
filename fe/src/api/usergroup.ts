import { api, type ApiResponse } from '@/utils/api'

// 用户组信息接口（扩展基础 UserGroup 类型）
export interface UserGroupInfo extends Models.UserGroup {
  userCount: number // 该用户组下的用户数量
}

// 添加用户组请求接口
export interface AddUserGroupRequest {
  name: string // 用户组名称，长度1-255位
}

export interface AddUserGroupResponse {
  id: number // 新创建用户组的ID
}

// 删除用户组请求接口
export interface DeleteUserGroupRequest {
  id: number // 用户组ID，必须大于1
}

// 修改用户组名称请求接口
export interface ModifyUserGroupNameRequest {
  id: number // 用户组ID
  name: string // 新的用户组名称
}

// 批量绑定文件到用户组请求接口
export interface BatchBindFilesRequest {
  groupId: number // 用户组ID
  fileIds: number[] // 文件ID列表，元素必须大于0；空数组表示清空绑定
}

// 获取用户组绑定文件响应接口
export interface GetBindFilesResponse {
  fileIds: number[] // 文件ID列表
}

// 用户组列表查询参数
export interface UserGroupListQuery {
  currentPage?: number // 当前页码，默认为1
  pageSize?: number // 每页大小，默认为10
  noPaginate?: boolean // 是否不分页，默认false
  name?: string // 用户组名称模糊搜索
}

// ===== 用户组管理接口 =====

// 添加用户组
export const addUserGroup = (
  data: AddUserGroupRequest
): Promise<ApiResponse<AddUserGroupResponse>> => {
  return api.post('/user_group/add', data).then((res) => res.data)
}

// 删除用户组
export const deleteUserGroup = (data: DeleteUserGroupRequest): Promise<ApiResponse> => {
  return api.post('/user_group/delete', data).then((res) => res.data)
}

// 获取用户组列表
export const getUserGroupList = (
  params?: UserGroupListQuery
): Promise<ApiResponse<Models.PaginationResponse<UserGroupInfo>>> => {
  return api.get('/user_group/list', { params }).then((res) => res.data)
}

// 修改用户组名称
export const modifyUserGroupName = (data: ModifyUserGroupNameRequest): Promise<ApiResponse> => {
  return api.post('/user_group/modify_name', data).then((res) => res.data)
}

// 批量绑定文件到用户组
export const batchBindFiles = (data: BatchBindFilesRequest): Promise<ApiResponse> => {
  return api.post('/user_group/batch_bind_files', data).then((res) => res.data)
}

// 获取用户组绑定的文件列表
export const getBindFiles = (groupId: number): Promise<ApiResponse<GetBindFilesResponse>> => {
  return api.get('/user_group/bind_files', { params: { groupId } }).then((res) => res.data)
}
