# -*- coding: utf8 -*-
from pysnmp.entity.rfc3413.oneliner import cmdgen


class SnmpApi(object):

    def __init__(self, host, port, version=1):
        cmdGen = cmdgen.CommandGenerator()
        self._cmd = cmdGen
        self._snmp_version = version
        if str(version) == '1':
            self._auth_data = cmdgen.CommunityData('public', mpModel=0)
        elif str(version) == '2c':
            self._auth_data = cmdgen.CommunityData('public')
        else:
            raise ValueError("unsupport version: {0}".format(version))

        self._target = cmdgen.UdpTransportTarget((host, port))
        self._extra_mib_path = None

    def _check_result(self, result):
        errorIndication, errorStatus, errorIndex, varBinds = result
        if errorIndication:
            pass
        else:
            if errorStatus:
                pass
            else:
                return varBinds
        return []

    def snmpwalk(self, MibVariable):
        if self._extra_mib_path is not None:
            MibVariable.addMibSource(self._extra_mib_path)
        result = self._cmd.nextCmd(
            self._auth_data,
            self._target,
            MibVariable
        )
        return self._check_result(result)


class ZxtmPoolStatic(SnmpApi):

    def __init__(self, host, port):
        SnmpApi.__init__(self, host, port)

    def _as_list(self, data_list):
        new_data = []
        for l in data_list:
            name, val = l[0]
            item = (name.getOid().prettyPrint().split('.', 14)[-1], val.prettyPrint())
            new_data.append(item)
        return new_data

    def _as_dict(self, data_list):
        new_data = {}
        for l in data_list:
            name, val = l[0]
            n, v = name.getOid().prettyPrint().split('.', 14)[-1], val.prettyPrint()
            new_data[n] = v
        return new_data

    def add_mib_source(self, path):
        self._extra_mib_path = path

    def get_pool_names(self):
        mib_variable = cmdgen.MibVariable('ZXTM-MIB', 'poolName')
        data = self.snmpwalk(mib_variable)
        return self._as_dict(data)

    def get_pool_bytes_in_lo(self):
        mib_variable = cmdgen.MibVariable('ZXTM-MIB', 'poolBytesInLo')
        data = self.snmpwalk(mib_variable)
        return self._as_dict(data)

    def get_pool_bytes_in_hi(self):
        mib_variable = cmdgen.MibVariable('ZXTM-MIB', 'poolBytesInHi')
        data = self.snmpwalk(mib_variable)
        return self._as_dict(data)

    def get_pool_bytes_out_lo(self):
        mib_variable = cmdgen.MibVariable('ZXTM-MIB', 'poolBytesOutLo')
        data = self.snmpwalk(mib_variable)
        return self._as_dict(data)

    def get_pool_bytes_out_hi(self):
        mib_variable = cmdgen.MibVariable('ZXTM-MIB', 'poolBytesOutHi')
        data = self.snmpwalk(mib_variable)
        return self._as_dict(data)
