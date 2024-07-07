import os
import re

# 定义正则表达式模式
pattern = r'\[INFO\] (\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6}) (\w+\.go:\d+): \[\d+\] the block is committed, No. of transactions: (\d+), view: (\d+), current view: (\d+), id: ([a-f0-9]+), forkNum: (-?\d+), Byz: (true|false), commitFromThis (true|false)'


# 获取log目录下的所有文件
log_dir = '/home/gpt/cbft/bamboo/bin/log'
html_dir = '/home/gpt/cbft/bamboo/bin/html'
log_files = [f for f in os.listdir(log_dir) if os.path.isfile(os.path.join(log_dir, f))]

for log_file in log_files:
    html = ''
    blocks = []
    # 打开日志文件
    with open(os.path.join(log_dir, log_file), 'r') as file:
        count = 0
        for line in file:
            if count == 1000:
                break
            # 使用正则表达式匹配行
            match = re.search(pattern, line)
            if match:
                # 提取信息
                date_time = match.group(1)
                file_info = match.group(2)
                transactions = match.group(3)
                view = match.group(4)
                current_view = match.group(5)
                block_id = match.group(6)
                fork_num = match.group(7)
                byz = match.group(8)
                commit_from_this = match.group(9)
                blocks.append({'view': int(view), 'byz': byz, 'id': block_id,'commit_from_this':commit_from_this})

    blocks.sort(key=lambda block: block['view'])
    blocksTrue = [block for block in blocks if block['commit_from_this'] == 'true']
    # print(blocks[:5])
    print(len(blocks)/ int(blocks[-1]['view']))
    print(len(blocksTrue)/blocks[-1]['view'])
    # 遍历排序后的blocks列表， 补充上被fork的区块
    current_view = 1
    for block in blocks:
        view = block['view']
        if view != current_view:
            for i in range(current_view, view):
                html += f'<div style="background-color: yellow">{current_view}fork</div>\n'
                current_view = current_view + 1

        current_view = current_view + 1
        byz = block['byz']
        color = "blue" if commit_from_this else "black"
        commit_from_this = block['commit_from_this']
        if byz == 'true':
            html += f'<div style="background-color: red">{view} <a style = "color: {color}">{commit_from_this}</a></div>\n'
        else:
            html += f'<div style="background-color: green">{view} <a style = "color:  {color}">{commit_from_this}</a></div>\n'

    html += """
      <style>
        div {
          display: inline-block;
          magin: 10px;
          height: 100px;
          width: 100px;
          padding: 10px;
          border: 1px solid black;
        }
      </style>
    """

    # 保存HTML代码到文件
    with open(os.path.join(html_dir, log_file.replace('.log', '.html')), 'w') as file:
        file.write(html)