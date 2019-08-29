import { Component, OnInit } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Router } from '@angular/router';
import { Location } from '@angular/common';
import { map } from 'rxjs/operators'
import { MatTableDataSource} from '@angular/material';
import { ConfigAssetLoaderService, Configuration} from '../config-asset-loader.service';


@Component({
  selector: 'app-pets',
  templateUrl: './pets.component.html',
  styleUrls: ['./pets.component.css']
})
export class PetsComponent implements OnInit {

  public pets: any[] = []
  public dataSource: MatTableDataSource<any>;

  public config: Configuration

  displayedColumns = ['name','kind','age','pic']

  constructor(private http: HttpClient, private router: Router, private location: Location, private configService: ConfigAssetLoaderService) {
    this.configService.loadConfigurations().subscribe(data =>   this.config = {
      petServiceUrl: (data as any).petServiceUrl,
      stage:  (data as any).stage,
    });
  }

  ngOnInit() {
    this.location.subscribe(() => {
      this.refresh();
    });
    this.refresh();
  }

  private refresh() {
    this.http.get(this.config.petServiceUrl)
      .pipe(map(result => result['Pets']))
      .subscribe(result => {
        this.pets = result;
        this.dataSource = new MatTableDataSource(this.pets)
      });
  }


}
