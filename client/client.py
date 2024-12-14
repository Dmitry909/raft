import requests

class RaftClient:
    def __init__(self, node):
        self.node = node

    def read(self, key):
        response = requests.get(self.node + "/read?key=" + key)
        if response.status_code != 200 and response.status_code != 404:
            raise Exception("read failed")
        if response.status_code == 200:
            assert "value" in response.json()
            return response.status_code, response.json()["value"]
        else:
            return response.status_code, ""

    def update(self, key, value):
        data = {
            "key": key,
            "value": value
        }
        response = requests.post(self.node + "/update", json=data)
        if response.status_code != 200:
            raise Exception("update failed")

    def delete(self, key):
        response = requests.delete(self.node + "/delete?key=" + key)
        if response.status_code != 200:
            raise Exception("delete failed")

    def current_role(self):
        response = requests.get(self.node + "/current_role")
        if response.status_code != 200:
            raise Exception("current_role failed")
        role = response.json()["role"]
        if role != "leader" and role != "candidate" and role != "follower":
            raise Exception("current_role returned incorrect response")
        return response.json()["role"]

    def stop(self):
        response = requests.post(self.node + "/stop")
        if response.status_code != 200:
            raise Exception("stop failed")

    def recover(self):
        response = requests.post(self.node + "/recover")
        if response.status_code != 200:
            raise Exception("recover failed")
