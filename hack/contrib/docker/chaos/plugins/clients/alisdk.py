import os
import sys

BASE_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

sys.path.insert(0, BASE_DIR + '/lib')

from utils.format import JSON, to_dict
from addict import Dict
import aliyun.api


class AliyunAPI(object):

    def __init__(self, conf=None, RegionId=None, *args, **kwargs):
        aliyun.setDefaultAppInfo('nMscVs3CaIXPEDUd', 'g4RWmftifuJxqUdqEWc69h0exO2V46')
        self.api = aliyun.api
        self.region_id = RegionId if RegionId is not None else ""

    def list_instances(self, RegionId=None, InstanceIds=None, dict_key=None):
        '''
        InstanceIds is a list
        '''
        m = self.api.Ecs20140526DescribeInstancesRequest()
        m.RegionId = self.region_id if RegionId is None else RegionId
        if InstanceIds is not None:
            m.InstanceIds = JSON.dumps(InstanceIds)

        response = m.getResponse()

        try:
            res_list = []
            for i in response['Instances']['Instance']:
                item = Dict({
                    "InstanceName": i['InstanceName'],
                    "InstanceId": i['InstanceId'],
                    "Ip": i['VpcAttributes']['PrivateIpAddress']['IpAddress'][0]
                })
                res_list.append(item)
            if dict_key is not None:
                return to_dict(res_list, dict_key)
            else:
                return res_list
        except Exception:
            return None

    def get_slb_backservers(self, LoadBalancerId, dict_key=None):
        m = self.api.Slb20140515DescribeLoadBalancerAttributeRequest()
        m.LoadBalancerId = LoadBalancerId
        response = m.getResponse()

        try:
            res_list = []
            for item in response['BackendServers']['BackendServer']:
                res_list.append(Dict(item))

            if dict_key is not None:
                return to_dict(res_list, dict_key)
            else:
                return res_list
        except Exception:
            return None

    def set_slb_backservers(self, LoadBalancerId, BackendServers):
        m = self.api.Slb20140515SetBackendServersRequest()
        m.LoadBalancerId = LoadBalancerId
        m.BackendServers = JSON.dumps(BackendServers)
        response = m.getResponse()
        return bool('Code' not in response)

    def get_slb_backserver_health(self, LoadBalancerId, dict_key=None):
        m = self.api.Slb20140515DescribeHealthStatusRequest()
        m.LoadBalancerId = LoadBalancerId
        response = m.getResponse()

        try:
            res_list = []
            for item in response['BackendServers']['BackendServer']:
                res_list.append(Dict(item))

            if dict_key is not None:
                return to_dict(res_list, dict_key)
            else:
                return res_list
        except Exception:
            return None
