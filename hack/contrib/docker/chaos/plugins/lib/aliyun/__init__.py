# -*- coding: utf-8 -*-
'''
Created on 2012-6-29

@author: lijie.ma
'''
from aliyun.api.base import sign



class appinfo(object):
    def __init__(self,accessKeyId,accessKeySecret):
        self.accessKeyId = accessKeyId
        self.accessKeySecret = accessKeySecret
        
def getDefaultAppInfo():
    pass

     
def setDefaultAppInfo(accessKeyId,accessKeySecret):
    default = appinfo(accessKeyId,accessKeySecret)
    global getDefaultAppInfo 
    getDefaultAppInfo = lambda: default
    




    

